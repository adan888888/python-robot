package monitor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/go-faster/errors"
	"github.com/gotd/contrib/auth/terminal"
	"github.com/gotd/log/logzap"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"testDockerTgbot/rebot"
)

type groupTarget struct {
	name   string
	chatID int64
}

func Run(conf rebot.Conf) error {
	if conf.Monitor.ApiID == 0 || conf.Monitor.ApiHash == "" {
		return errors.New("请在 config.yaml 的 monitor 段填写 apiId 和 apiHash（https://my.telegram.org/apps）")
	}

	groups := conf.MonitorGroups()
	if len(groups) == 0 {
		return errors.New("请在 config.yaml 的 monitor 段填写 groups，或 groupName / groupChatId")
	}

	taskFile := rebot.ResolveTaskFilePath(conf.TaskFilePath())
	sessionPath := strings.TrimSpace(conf.Monitor.Session)
	if sessionPath == "" {
		sessionPath = "monitor/session.json"
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		return errors.Wrap(err, "create session dir")
	}

	log, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	defer log.Sync() //nolint:errcheck

	tdLog := logzap.New(log)
	for _, g := range groups {
		log.Info("监听群",
			zap.String("groupName", g.Name),
			zap.Int64("groupChatId", g.ChatID),
		)
	}
	log.Info("监听配置",
		zap.String("taskFile", taskFile),
		zap.String("session", sessionPath),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	dispatcher := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
		Logger:  logzap.New(log.Named("gaps")),
	})

	client := telegram.NewClient(conf.Monitor.ApiID, conf.Monitor.ApiHash, telegram.Options{
		Logger:         tdLog,
		SessionStorage: &session.FileStorage{Path: sessionPath},
		UpdateHandler:  gaps,
	})

	var targets []groupTarget

	handleMessage := func(ctx context.Context, entities tg.Entities, msg tg.MessageClass) error {
		m, ok := msg.(*tg.Message)
		if !ok {
			return nil
		}
		groupName, matched := matchesAnyTarget(entities, m.PeerID, targets)
		if !matched {
			return nil
		}

		text := strings.TrimSpace(m.Message)
		if text == "" {
			text = "[非文本消息]"
		}

		senderName := formatSender(entities, m)
		if err := rebot.AppendTaskGroupMessage(taskFile, groupName, senderName, text); err != nil {
			log.Error("写入工作任务文件失败", zap.Error(err))
			return nil
		}
		log.Info("已记录群消息", zap.String("group", groupName), zap.String("from", senderName), zap.String("text", text))
		return nil
	}

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		return handleMessage(ctx, e, u.Message)
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		return handleMessage(ctx, e, u.Message)
	})

	flow := auth.NewFlow(terminal.OS(), auth.SendCodeOptions{})

	return client.Run(ctx, func(ctx context.Context) error {
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return errors.Wrap(err, "auth")
		}

		self, err := client.Self(ctx)
		if err != nil {
			return errors.Wrap(err, "get self")
		}

		targets, err = resolveGroupTargets(ctx, client.API(), groups)
		if err != nil {
			return err
		}
		for _, t := range targets {
			log.Info("已解析监听群", zap.String("groupName", t.name), zap.Int64("groupChatId", t.chatID))
		}

		name := self.FirstName
		if self.Username != "" {
			name = fmt.Sprintf("%s (@%s)", name, self.Username)
		}
		log.Info("个人账号已登录，开始监听群消息", zap.String("user", name), zap.Int("groupCount", len(targets)))

		return gaps.Run(ctx, client.API(), self.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				log.Info("更新同步已启动")
			},
		})
	})
}

func resolveGroupTargets(ctx context.Context, api *tg.Client, groups []rebot.MonitorGroup) ([]groupTarget, error) {
	needLookup := false
	for _, g := range groups {
		if g.ChatID == 0 && strings.TrimSpace(g.Name) != "" {
			needLookup = true
			break
		}
	}

	var chats []tg.ChatClass
	if needLookup {
		result, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      100,
		})
		if err != nil {
			return nil, errors.Wrap(err, "get dialogs")
		}
		modified, ok := result.AsModified()
		if !ok {
			return nil, errors.New("无法获取会话列表")
		}
		chats = modified.GetChats()
	}

	targets := make([]groupTarget, 0, len(groups))
	for _, g := range groups {
		name := strings.TrimSpace(g.Name)
		chatID := g.ChatID
		if chatID == 0 {
			if name == "" {
				return nil, errors.New("监听群需填写 name 或 chatId")
			}
			id, err := findGroupChatIDInChats(chats, name)
			if err != nil {
				return nil, err
			}
			chatID = id
		}
		if name == "" {
			name = fmt.Sprintf("chat:%d", chatID)
		}
		targets = append(targets, groupTarget{name: name, chatID: chatID})
	}
	return targets, nil
}

func findGroupChatIDInChats(chats []tg.ChatClass, title string) (int64, error) {
	for _, chat := range chats {
		switch c := chat.(type) {
		case *tg.Chat:
			if strings.EqualFold(c.Title, title) {
				return -int64(c.ID), nil
			}
		case *tg.Channel:
			if strings.EqualFold(c.Title, title) {
				return -(1_000_000_000_000 + c.ID), nil
			}
		}
	}
	return 0, errors.Errorf("未找到名为 %q 的群，请确认个人账号已加入该群", title)
}

func matchesAnyTarget(entities tg.Entities, peer tg.PeerClass, targets []groupTarget) (string, bool) {
	for _, t := range targets {
		if matchesTargetGroup(entities, peer, t.name, t.chatID) {
			return t.name, true
		}
	}
	return "", false
}

func formatSender(entities tg.Entities, m *tg.Message) string {
	if sender, ok := resolveSender(entities, m); ok {
		if sender.Bot && sender.Username != "" {
			return "@" + sender.Username
		}
		if sender.Username != "" {
			return "@" + sender.Username
		}
		name := strings.TrimSpace(sender.FirstName + " " + sender.LastName)
		if name != "" {
			return name
		}
	}

	if author, ok := m.GetPostAuthor(); ok && author != "" {
		return author
	}

	return "unknown"
}

func resolveSender(entities tg.Entities, m *tg.Message) (*tg.User, bool) {
	fromID, ok := m.GetFromID()
	if !ok {
		return nil, false
	}
	peerUser, ok := fromID.(*tg.PeerUser)
	if !ok {
		return nil, false
	}
	user, ok := entities.Users[peerUser.UserID]
	if !ok {
		return nil, false
	}
	return user, true
}

func matchesTargetGroup(entities tg.Entities, peer tg.PeerClass, groupName string, groupChatID int64) bool {
	if groupChatID != 0 {
		return peerMatchesChatID(peer, groupChatID)
	}
	if groupName == "" {
		return true
	}
	title, ok := chatTitleFromPeer(entities, peer)
	return ok && strings.EqualFold(title, groupName)
}

func chatTitleFromPeer(entities tg.Entities, peer tg.PeerClass) (string, bool) {
	switch p := peer.(type) {
	case *tg.PeerChat:
		chat, ok := entities.Chats[p.ChatID]
		if !ok {
			return "", false
		}
		return chat.Title, true
	case *tg.PeerChannel:
		channel, ok := entities.Channels[p.ChannelID]
		if !ok {
			return "", false
		}
		return channel.Title, true
	default:
		return "", false
	}
}

func peerMatchesChatID(peer tg.PeerClass, want int64) bool {
	got, ok := botAPIChatID(peer)
	if !ok {
		return false
	}
	return got == want
}

func botAPIChatID(peer tg.PeerClass) (int64, bool) {
	switch p := peer.(type) {
	case *tg.PeerChat:
		return -int64(p.ChatID), true
	case *tg.PeerChannel:
		return -(1_000_000_000_000 + int64(p.ChannelID)), true
	default:
		return 0, false
	}
}
