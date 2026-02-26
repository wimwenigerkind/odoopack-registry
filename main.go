package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/config"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
)

func main() {
	config.LoadConfig()

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
