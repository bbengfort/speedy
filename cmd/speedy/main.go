package main

import (
	"crypto/rand"
	"os"

	speedy "github.com/bbengfort/speedy/pkg"
	"github.com/bbengfort/speedy/pkg/client"
	"github.com/bbengfort/speedy/pkg/config"
	"github.com/bbengfort/speedy/pkg/server"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
)

func main() {
	// Load .env file into environment
	godotenv.Load()

	// Create a multi-command CLI application
	app := cli.NewApp()
	app.Name = "speedy"
	app.Version = speedy.Version()
	app.Usage = "a simple http/2 chat server and client"
	app.Commands = []*cli.Command{
		{
			Name:   "serve",
			Action: serve,
			Usage:  "serve the speedy chat server",
		},
		{
			Name:   "chat",
			Action: chat,
			Usage:  "open the speedy chat client",
		},
	}

	// Run the CLI application
	app.Run(os.Args)
}

func serve(*cli.Context) (err error) {
	var conf config.ServerConfig
	if conf, err = config.Server(); err != nil {
		return cli.Exit(err, 1)
	}

	var srv *server.Server
	if srv, err = server.New(conf); err != nil {
		return cli.Exit(err, 1)
	}

	return srv.Serve()
}

func chat(*cli.Context) (err error) {
	var conf config.ClientConfig
	if conf, err = config.Client(); err != nil {
		return cli.Exit(err, 1)
	}

	var api *client.Client
	if api, err = client.New(conf); err != nil {
		return cli.Exit(err, 1)
	}

	data := make([]byte, 1231234)
	rand.Read(data)

	if err = api.Post(data); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}
