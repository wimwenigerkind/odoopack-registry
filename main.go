package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("server_address", "0.0.0.0:6969")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("no config file found, using defaults")
	}

	r := gin.Default()

	err := r.Run(viper.GetString("server_address"))
	if err != nil {
		panic(err)
	}
}
