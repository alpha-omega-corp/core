package app

import (
	"context"
	"embed"
	"fmt"
	"github.com/alpha-omega-corp/core/app/models"
	"github.com/alpha-omega-corp/core/app/proto"
	"github.com/uptrace/bun/dbfixture"
	"github.com/uptrace/bun/migrate"
	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/bunrouterotel"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type App struct {
	name          string
	dbHandler     *StorageHandler
	configHandler *ConfigHandler
	config        *Config
	models        []any

	fs embed.FS
}

func NewApp(efs embed.FS) *App {
	return &App{
		fs: efs,
	}
}

// Bootstrap /* Channel to keep connection alive */
func (app *App) Bootstrap(m ...any) os.Signal {
	app.models = append(app.models, append(m,
		(*models.UserToRole)(nil),
		(*models.User)(nil),
		(*models.Role)(nil),
		(*models.Service)(nil),
		(*models.Permission)(nil),
	)...)

	appCli := &cli.Command{
		Usage: "application cli",
		Commands: []*cli.Command{
			app.serverCommand(),
			app.migrationCommand(),
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

func (app *App) serverCommand() *cli.Command {
	return app.createCommand("app", "server", func(ctx context.Context, cmd *cli.Command) {
		go func() {
			if err := GRPC("localhost:50050", func(grpc *grpc.Server) {
				proto.RegisterAuthServiceServer(grpc, NewAuthServer(app.dbHandler.Database(), NewAuthWrapper(app.config.Secret)))
			}); err != nil {
				panic(err)
			}
		}()

		HTTP(app.config, func(r *bunrouter.Router) {
			r.Use(bunrouterotel.NewMiddleware())
			r.Use(NewCorsMiddleware())

			//NewAuthClient(r)
		})
	})
}

func (app *App) migrationCommand() *cli.Command {
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

func (app *App) createCommand(category string, name string, action func(ctx context.Context, cmd *cli.Command)) *cli.Command {
	return &cli.Command{
		Name:     name,
		Category: category,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := app.loadConfig(); err != nil {
				panic(err)
			}

			config, err := app.configHandler.DefaultConfig()
			if err != nil {
				return err
			}

			app.config = config
			app.dbHandler = NewStorageHandler(app.config.Dsn)
			app.dbHandler.Database().RegisterModel(app.models...)

			action(ctx, cmd)

			return nil
		},
	}
}

func (app *App) loadConfig() error {
	app.configHandler = NewConfigHandler("http://localhost:2379")

	return fs.WalkDir(app.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		file, err := app.fs.ReadFile(path)
		if err != nil {
			return err
		}

		if err := app.configHandler.Write(context.Background(), path, file); err != nil {
			return err
		}

		app.configHandler.paths = append(app.configHandler.paths, path)

		return nil
	})
}

/*

func (app *App) CreateHttp(init func(configHandler *ConfigHandler, router *bunrouter.Router)) *App {
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

	return app
}
func (app *App) CreateGrpc(init func(config *Config, db *bun.DB, grpc *grpc.Server), models ...any) *App {
	app.models = append(app.models, models...)

	appCli := &cli.Command{
		Usage: "cloud application cli",
		Commands: []*cli.Command{
			app.newGrpcCommand(init),
			app.migrationCommand(),
		},
	}

	if err := appCli.Run(context.Background(), os.Args); err != nil {
		log.Fatalf("app start error: %v\n", err)
	}

	return app
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

		app.loadConfig()

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
*/
