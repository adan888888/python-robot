package rebot

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Conf struct {
	System struct {
		Name string `yaml:"name"`
	} `yaml:"system"`

	TgBot struct {
		Token        string `yaml:"Token"`
		BotUsername  string `yaml:"botUsername"`
		TaskFilePath string `yaml:"taskFile"`
	} `yaml:"tgbot"`

	Monitor struct {
		Enabled     bool           `yaml:"enabled"`
		ApiID       int            `yaml:"apiId"`
		ApiHash     string         `yaml:"apiHash"`
		Phone       string         `yaml:"phone"`
		GroupName   string         `yaml:"groupName"`   // 兼容旧配置，与 groupChatId 成对使用
		GroupChatID int64          `yaml:"groupChatId"` // 兼容旧配置
		Groups      []MonitorGroup `yaml:"groups"`
		TaskFile    string         `yaml:"taskFile"`
		Session     string         `yaml:"session"`
	} `yaml:"monitor"`
}

type MonitorGroup struct {
	Name   string `yaml:"name"`
	ChatID int64  `yaml:"chatId"`
}

func (c Conf) MonitorGroups() []MonitorGroup {
	if len(c.Monitor.Groups) > 0 {
		return c.Monitor.Groups
	}
	if c.Monitor.GroupName != "" || c.Monitor.GroupChatID != 0 {
		return []MonitorGroup{{
			Name:   c.Monitor.GroupName,
			ChatID: c.Monitor.GroupChatID,
		}}
	}
	return nil
}

func LoadConf(path string) (Conf, error) {
	var conf Conf
	file, err := os.ReadFile(path)
	if err != nil {
		return conf, err
	}
	err = yaml.Unmarshal(file, &conf)
	return conf, err
}

func (c Conf) TaskFilePath() string {
	if c.Monitor.TaskFile != "" {
		return c.Monitor.TaskFile
	}
	if c.TgBot.TaskFilePath != "" {
		return c.TgBot.TaskFilePath
	}
	return ""
}

