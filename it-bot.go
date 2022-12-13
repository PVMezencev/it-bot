package main

import (
	"main/bot"
	"path/filepath"

	rmq "main/rabbitmq-client"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func initConfig(configDir string) error {
	const configFileName = "config"
	logrus.Infof("конфигурационный файл: %s", filepath.Join(configDir, configFileName))

	viper.AddConfigPath(configDir)
	viper.SetConfigName(configFileName)
	return viper.ReadInConfig()
}

func main() {
	logrus.SetFormatter(new(logrus.TextFormatter))
	if err := initConfig("configs"); err != nil {
		logrus.Fatalf("ошибка инициализации конфигурации приложения: %s", err.Error())
	}

	botConsumers := make([]interface{}, 0)

	rmqc := rmq.NewRMQ(rmq.NewRMQCreds(
		viper.GetString("rabbitmq.username"),
		viper.GetString("rabbitmq.password"),
		viper.GetString("rabbitmq.host"),
		viper.GetString("rabbitmq.port"),
	))

	if rmqc != nil {
		botConsumers = append(botConsumers, rmqc)
	}

	// Инициализируем телеграм-бота.
	tBot := bot.NewTBot(bot.TBotConfig{
		Token:       viper.GetString("telegram.bot_token"),
		SystemChats: viper.GetStringSlice("telegram.system_chats"),
		AdminChats:  viper.GetStringSlice("telegram.admin_chats"),
	})
	if tBot != nil {
		tBot.Start(botConsumers...)
	}
}
