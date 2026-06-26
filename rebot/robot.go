package rebot

import (
	"fmt"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

func TgRobot(config Conf) {
	var bot, err = tgbotapi.NewBotAPI(config.TgBot.Token)
	if err != nil {
		panic(err)
	}
	//bot.Debug = true

	expectedBotUsername := strings.TrimPrefix(strings.TrimSpace(config.TgBot.BotUsername), "@")
	if expectedBotUsername != "" && !strings.EqualFold(bot.Self.UserName, expectedBotUsername) {
		log.Warnf("配置 botUsername=%s 与当前 Token 对应机器人 @%s 不一致，请确认 config.yaml 中的 Token", expectedBotUsername, bot.Self.UserName)
	}

	taskFile := strings.TrimSpace(config.TgBot.TaskFilePath)
	if taskFile == "" {
		taskFile = strings.TrimSpace(os.Getenv("TASK_FILE"))
	}
	taskFile = ResolveTaskFilePath(taskFile)
	log.Infof("机器人 @%s 发出的消息将写入: %s", bot.Self.UserName, taskFile)

	// 存储用户的选择
	userSelections := make(map[int64][]string)

	u := tgbotapi.NewUpdate(0) //创建了一个新的更新对象 u，用于从 Telegram 服务器获取消息更新。参数 0 表示从最早的未读消息开始获取更新.
	u.Timeout = 60             //60秒内没有消息更新，就停止轮询，以节约资源
	//u.Offset = -1              // 跳过旧的更新

	// 获取一个监听管道，进行轮询监听飞机消息
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		//if update.Message == nil { // 忽略任何非消息更新 //会把update.CallbackQuery消息过滤掉
		//	continue
		//}
		// 打印收到的消息
		if update.Message != nil {
			log.Infof("收到消息==>[userName=%s/From.String=%s/ID=%d] [消息是=%s}] [Chat.ID=%v] ", update.Message.From.UserName, update.Message.From.String(), update.Message.From.ID, update.Message.Text, update.Message.Chat.ID) //如果没有设置userName,From.String()可以取到name

			// 若群里出现该机器人发出的消息（非本进程 Send 触发时），也一并记录
			if update.Message.From != nil && update.Message.From.IsBot &&
				strings.EqualFold(update.Message.From.UserName, bot.Self.UserName) &&
				update.Message.Text != "" {
				recordBotOutgoing(taskFile, bot.Self.UserName, update.Message.Text)
			}
		}

		if update.CallbackQuery != nil { // 用户点击按钮
			log.Printf("CallbackQuery received: %+v", update.CallbackQuery)
			callback := update.CallbackQuery
			chatID := callback.Message.Chat.ID
			data := callback.Data

			if data == "confirm" { // 用户点击确认
				selection := userSelections[chatID]
				if len(selection) == 0 {
					reply := "你未选择任何选项！"
					sendBotMessage(bot, taskFile, bot.Self.UserName, tgbotapi.NewMessage(chatID, reply))
				} else {
					reply := fmt.Sprintf("你选择了: %s", strings.Join(selection, ", "))
					sendBotMessage(bot, taskFile, bot.Self.UserName, tgbotapi.NewMessage(chatID, reply))
				}
				userSelections[chatID] = nil // 清空用户选择
			} else { // 记录选择
				userSelections[chatID] = toggleSelection(userSelections[chatID], data)
				//reply := fmt.Sprintf("当前选择: %s", strings.Join(userSelections[chatID], ", "))
				//bot.Send(tgbotapi.NewMessage(chatID, reply))
			}

			// 回答 CallbackQuery
			bot.Send(tgbotapi.NewCallback(callback.ID, "操作已记录"))
		}

		if update.Message != nil && update.Message.IsCommand() { // 用户发送命令
			msg := update.Message
			chatID := msg.Chat.ID

			switch msg.Command() {
			case "start":
				// 发送选项按钮
				buttons := []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData("选项1", "option1"),
					tgbotapi.NewInlineKeyboardButtonData("选项2", "option2"),
					tgbotapi.NewInlineKeyboardButtonData("选项3", "option3"),
				}
				confirmButton := tgbotapi.NewInlineKeyboardButtonData("确认", "confirm")

				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(buttons...),
					tgbotapi.NewInlineKeyboardRow(confirmButton),
				)

				reply := tgbotapi.NewMessage(chatID, "请选择选项：")
				reply.ReplyMarkup = keyboard

				_, err = sendBotMessage(bot, taskFile, bot.Self.UserName, reply)
				if err != nil {
					panic(err)
				}
			}
		}

		// 检查消息是否提到了机器人 或者是命令
		if update.Message != nil && (update.Message.IsCommand() || strings.Contains(update.Message.Text, "@"+bot.Self.UserName)) {
			//if strings.HasPrefix(update.Message.Text, "@bx_xia_Bot") {
			// 回复消息
			responseText := "你提到我了吗？我在这里！大佬请指教！"
			//msg := tgbotapi.NewMessage(update.Message.Chat.ID, responseText)
			msg := tgbotapi.NewMessage(config.TgBot.ChatID, responseText)
			switch update.Message.Command() {
			case "start":
				msg.Text = "你好，发送 /help 查看可用命令"
			case "help":
				msg.Text = "You can control me by sending these commands:\n/start - to start the bot\n/help - to get this help message"
			default:
				if strings.Contains(update.Message.Text, "@"+bot.Self.UserName+" ") { //@我的(机器人)
					msg.Text = "不要@我，我很忙..."
				} else {
					msg.Text = "请重新输入..."
				}
			}
			//msg.ReplyToMessageID = update.Message.MessageID  加这个是回复消息
			// 发送回复消息
			sendBotMessage(bot, taskFile, bot.Self.UserName, msg)
		} else {
			//如果是#号开头，就是我要发到群里的消息
			if update.Message != nil && strings.HasPrefix(update.Message.Text, "#") {
				msg := tgbotapi.NewMessage(config.TgBot.ChatID, update.Message.Text)
				sendBotMessage(bot, taskFile, bot.Self.UserName, msg)
			}
		}

	}
}

// toggleSelection 用于更新用户的选择
func toggleSelection(selection []string, item string) []string {
	for i, v := range selection {
		if v == item {
			// 如果已经选中，取消选择
			return append(selection[:i], selection[i+1:]...)
		}
	}
	// 如果未选中，添加到选择中
	return append(selection, item)
}
