package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"testDockerTgbot/monitor"
	"testDockerTgbot/rebot"
)

// https://www.bilibili.com/video/BV1HH4y1W7H4/?spm_id_from=333.337.search-card.all.click&vd_source=55f7073cc1049edc8b91cea83217e7b6 视频
// https://www.fengfengzhidao.com/article/dtyibo4BEG4v2tWkcxXp 文档
func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	conf, err := rebot.LoadConf(configPath)
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}

	if conf.Monitor.Enabled {
		go func() {
			log.Println("启动群消息监听...")
			if err := monitor.Run(conf); err != nil {
				log.Printf("群消息监听退出: %v", err)
			}
		}()
	}

	log.Println("启动 Telegram 机器人...")
	go rebot.TgRobot(conf)

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code": 0,
			"msg":  "看到消息就说明布置成功了。",
			"data": gin.H{
				"tiem":    time.Now().Format("2006-01-02 15:04:05"),
				"token":   conf.TgBot.Token,
				"name":    conf.System.Name,
				"chatId":  conf.TgBot.ChatID,
				"monitor": conf.Monitor.Enabled,
			},
		})
	})

	if err := r.Run(":5000"); err != nil {
		log.Fatal(err)
	}
}
