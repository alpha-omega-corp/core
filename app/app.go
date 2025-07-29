package app

import (
	"context"
	"embed"
	"fmt"
	"github.com/alpha-omega-corp/core/app/models"
	"github.com/alpha-omega-corp/core/app/proto"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dbfixture"
	"github.com/uptrace/bun/migrate"
	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/bunrouterotel"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type App struct {
	name          string
	dbHandler     *StorageHandler
	configHandler *ConfigHandler
	models        []any

	fs embed.FS
}

func NewApp(efs embed.FS, name string) *App {
	return &App{
		name: name,
		fs:   efs,
	}
}

func (app *App) CreateApi(init func(configHandler *ConfigHandler, router *bunrouter.Router)) os.Signal {
	appCli := &cli.Command{
		Usage: "cloud application cli",
		Commands: []*cli.Command{
			app.newHttpCommand(init),
		},
	}

	if err := appCli.Run(context.Background(), os.Args); err != nil {
		log.Fatalf("app start error: %v\n", err)
	}

	// Create keyboard listener
	ch := make(chan os.Signal, 3)
	signal.Notify(
		ch,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)

	return <-ch
}

func (app *App) CreateApp(init func(config *Config, db *bun.DB, grpc *grpc.Server), models ...any) {
	app.models = append(app.models, models...)

	appCli := &cli.Command{
		Usage: "cloud application cli",
		Commands: []*cli.Command{
			app.newGrpcCommand(init),
			app.migrateCommand(),
		},
	}

	if err := appCli.Run(context.Background(), os.Args); err != nil {
		log.Fatalf("app start error: %v\n", err)
	}
}

func (app *App) newGrpcCommand(init func(config *Config, db *bun.DB, grpc *grpc.Server)) *cli.Command {
	return app.createCommand("app", "server", func(ctx context.Context, cmd *cli.Command) {
		if err := GRPC(*app.configHandler, app.dbHandler, func(db *bun.DB, grpc *grpc.Server) {
			init(app.configHandler.config, db, grpc)
		}); err != nil {
			panic(err)
		}
	})
}

func (app *App) newHttpCommand(init func(configHandler *ConfigHandler, router *bunrouter.Router)) *cli.Command {
	return app.createCommand("app", "server", func(ctx context.Context, cmd *cli.Command) {

		app.models = append(app.models, []interface{}{
			(*models.UserToRole)(nil),
			(*models.User)(nil),
			(*models.Role)(nil),
			(*models.Service)(nil),
			(*models.Permission)(nil),
		}...)

		env := cmd.String("env")
		app.loadConfig(env, app.name)

		if app.configHandler.config.Dsn != nil {
			app.dbHandler = NewStorageHandler(*app.configHandler.config.Dsn)
			app.dbHandler.Database().RegisterModel(app.models...)
		}

		userURL := app.configHandler.config.Env.GetString("user_url")
		userDSN := app.configHandler.config.Env.GetString("user_dsn")

		userConfigHandler := &ConfigHandler{
			name: "user",
			config: &Config{
				Url: &userURL,
				Dsn: &userDSN,
				Env: app.configHandler.config.Env,
			},
		}

		fmt.Println(app.configHandler.config.Url)
		*userConfigHandler.config.Dsn = userConfigHandler.config.Env.GetString("user_dsn")

		go func() {
			if err := GRPC(*userConfigHandler, app.dbHandler, func(db *bun.DB, grpc *grpc.Server) {
				auth := NewAuthWrapper(userConfigHandler.config.Env.GetString("user_secret"))
				proto.RegisterAuthServiceServer(grpc, NewAuthServer(db, auth))
			}); err != nil {
				panic(err)
			}
		}()

		fmt.Print(*app.configHandler.config.Url)

		HTTP(*app.configHandler, func(r *bunrouter.Router) {
			r.Use(bunrouterotel.NewMiddleware())
			r.Use(NewCorsMiddleware())

			// Register user client
			userClient := RegisterAuthClient(NewAuthClient(NewClient(userConfigHandler.config, proto.NewAuthServiceClient)), r)
			r.Use(NewAuthMiddleware(userClient).Auth)

			init(app.configHandler, r)
		})

	})
}

func (app *App) migrateCommand() *cli.Command {
	return app.createCommand("db", "migration", func(ctx context.Context, cmd *cli.Command) {
		db := app.dbHandler.Database()

		migrator := migrate.NewMigrator(db, migrate.NewMigrations())
		if err := migrator.Init(ctx); err != nil {
			panic(err)
		}
		if err := db.ResetModel(ctx, app.models...); err != nil {
			panic(err)
		}

		fixture := dbfixture.New(db)
		if err := fixture.Load(ctx, os.DirFS("cmd/fixtures"), "fixture.yml"); err != nil {
			fmt.Printf("load fixture error: %v\n", err)
			panic(err)
		}
	})
}

func (app *App) loadConfig(env string, name string) {
	configFile, err := app.fs.ReadFile(GetConfigPath(env))
	if err != nil {
		log.Fatalf("read config file error: %v\n", err)
	}

	app.configHandler = NewConfigHandler(context.Background(), name, configFile)
}

func (app *App) createCommand(category string, name string, action func(ctx context.Context, cmd *cli.Command)) *cli.Command {
	return &cli.Command{
		Name:     name,
		Category: category,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Value:   "local",
				Usage:   "environment to select configuration file",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			env := cmd.String("env")
			app.loadConfig(env, app.name)

			if app.configHandler.config.Dsn != nil {
				app.dbHandler = NewStorageHandler(*app.configHandler.config.Dsn)
				app.dbHandler.Database().RegisterModel(app.models...)
			}

			action(ctx, cmd)

			return nil
		},
	}
}
