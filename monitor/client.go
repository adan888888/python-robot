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
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/gotd/log/logzap"
	"go.uber.org/zap"

	"testDockerTgbot/rebot"
)

func Run(conf rebot.Conf) error {
	if conf.Monitor.ApiID == 0 || conf.Monitor.ApiHash == "" {
		return errors.New("请在 config.yaml 的 monitor 段填写 apiId 和 apiHash（https://my.telegram.org/apps）")
	}

	watchBot := strings.TrimPrefix(strings.TrimSpace(conf.Monitor.WatchBot), "@")
	if watchBot == "" {
		watchBot = "toki999999_bot"
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
	log.Info("监听配置",
		zap.String("watchBot", watchBot),
		zap.Int64("groupChatId", conf.Monitor.GroupChatID),
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

	handleMessage := func(ctx context.Context, entities tg.Entities, msg tg.MessageClass) error {
		m, ok := msg.(*tg.Message)
		if !ok {
			return nil
		}
		if conf.Monitor.GroupChatID != 0 && !peerMatchesChatID(m.PeerID, conf.Monitor.GroupChatID) {
			return nil
		}

		sender, ok := resolveSender(entities, m)
		if !ok || !sender.Bot {
			return nil
		}
		if !strings.EqualFold(sender.Username, watchBot) {
			return nil
		}

		text := strings.TrimSpace(m.Message)
		if text == "" {
			text = "[非文本消息]"
		}

		senderName := "@" + sender.Username
		if err := rebot.AppendTaskMessage(taskFile, senderName, text); err != nil {
			log.Error("写入工作任务文件失败", zap.Error(err))
			return nil
		}
		log.Info("已记录机器人消息", zap.String("from", senderName), zap.String("text", text))
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

		name := self.FirstName
		if self.Username != "" {
			name = fmt.Sprintf("%s (@%s)", name, self.Username)
		}
		log.Info("个人账号已登录，开始监听群里机器人消息", zap.String("user", name))

		return gaps.Run(ctx, client.API(), self.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				log.Info("更新同步已启动")
			},
		})
	})
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

func peerMatchesChatID(peer tg.PeerClass, want int64) bool {
	if want == 0 {
		return true
	}
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
