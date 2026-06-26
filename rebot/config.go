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
		ChatID       int64  `yaml:"chatID"`
		BotUsername  string `yaml:"botUsername"`
		TaskFilePath string `yaml:"taskFile"`
	} `yaml:"tgbot"`

	Monitor struct {
		Enabled     bool   `yaml:"enabled"`
		ApiID       int    `yaml:"apiId"`
		ApiHash     string `yaml:"apiHash"`
		Phone       string `yaml:"phone"`
		WatchBot    string `yaml:"watchBot"`
		GroupChatID int64  `yaml:"groupChatId"`
		TaskFile    string `yaml:"taskFile"`
		Session     string `yaml:"session"`
	} `yaml:"monitor"`
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

