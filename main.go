package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
)

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("server_address", "0.0.0.0:6969")

	viper.SetDefault("db.host", "localhost")
	viper.SetDefault("db.port", "5432")
	viper.SetDefault("db.user", "postgres")
	viper.SetDefault("db.password", "postgres")
	viper.SetDefault("db.name", "odoopack-registry")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("no config file found, using defaults")
	}

	db, err := database.Connect()
	if err != nil {
		panic(err)
	}

	err = database.Migrate(db)
	if err != nil {
		panic(err)
	}

	r := gin.Default()

	fmt.Printf("starting on: http://%v\n", viper.GetString("server_address"))

	err = r.Run(viper.GetString("server_address"))
	if err != nil {
		panic(err)
	}
}
