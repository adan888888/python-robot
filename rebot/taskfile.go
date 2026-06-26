package rebot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

func defaultTaskFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "工作任务.txt"
	}
	return filepath.Join(home, "Documents", "工作任务.txt")
}

func ResolveTaskFilePath(configured string) string {
	if configured != "" {
		return configured
	}
	return defaultTaskFilePath()
}

func AppendTaskMessage(filePath, sender, text string) error {
	return AppendTaskGroupMessage(filePath, "", sender, text)
}

func AppendTaskGroupMessage(filePath, group, sender, text string) error {
	if text == "" {
		return nil
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create task file dir: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open task file: %w", err)
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05")
	var line string
	if group != "" {
		line = fmt.Sprintf("[%s] [%s] %s: %s\n", ts, group, sender, text)
	} else {
		line = fmt.Sprintf("[%s] %s: %s\n", ts, sender, text)
	}
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}
	return nil
}

func recordBotOutgoing(taskFile, botUsername, text string) {
	if text == "" {
		return
	}
	sender := botUsername
	if sender != "" && !strings.HasPrefix(sender, "@") {
		sender = "@" + sender
	}
	if err := AppendTaskMessage(taskFile, sender, text); err != nil {
		log.Errorf("写入工作任务文件失败: %v", err)
	} else {
		log.Infof("已记录机器人消息: %s", text)
	}
}

func sendBotMessage(bot *tgbotapi.BotAPI, taskFile, botUsername string, msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	sent, err := bot.Send(msg)
	if err != nil {
		return sent, err
	}
	if sent.Text != "" {
		recordBotOutgoing(taskFile, botUsername, sent.Text)
	}
	return sent, err
}
