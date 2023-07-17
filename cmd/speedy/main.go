package main

import (
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
	app.Usage = "a simple http/2 pubsub server and client"
	app.Commands = []*cli.Command{
		{
			Name:   "serve",
			Action: serve,
			Usage:  "serve the speedy chat server",
		},
		{
			Name:   "pub",
			Action: pub,
			Usage:  "publish messages",
		},
		{
			Name:   "sub",
			Action: sub,
			Usage:  "subscribe to messages",
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

func pub(*cli.Context) (err error) {
	var conf config.ClientConfig
	if conf, err = config.Client(); err != nil {
		return cli.Exit(err, 1)
	}

	var api *client.Client
	if api, err = client.New(conf); err != nil {
		return cli.Exit(err, 1)
	}

	if err = api.Publish(os.Stdin); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func sub(*cli.Context) (err error) {
	var conf config.ClientConfig
	if conf, err = config.Client(); err != nil {
		return cli.Exit(err, 1)
	}

	var api *client.Client
	if api, err = client.New(conf); err != nil {
		return cli.Exit(err, 1)
	}

	if err = api.Subscribe(os.Stdout); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}
