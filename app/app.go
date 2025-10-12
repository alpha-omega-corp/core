package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/alpha-omega-corp/core/app/models"
	"github.com/alpha-omega-corp/core/app/proto"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dbfixture"
	"github.com/uptrace/bun/migrate"
	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/bunrouterotel"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"
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

func CreateClient[T any](serviceConstructor func(conn grpc.ClientConnInterface) T) T {
	return NewClient("localhost:50051", serviceConstructor).Service()
}

// Bootstrap /* Channel to keep the connection alive */
func (app *App) Bootstrap(start func(db *bun.DB, router *bunrouter.Router, conn *grpc.Server), m ...any) os.Signal {
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
			app.serverCommand(start),
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

func (app *App) serverCommand(start func(db *bun.DB, router *bunrouter.Router, conn *grpc.Server)) *cli.Command {
	return app.createCommand("app", "server", func(ctx context.Context, cmd *cli.Command) {
		CreateHTTP(app.config, func(router *bunrouter.Router) {
			router.Use(bunrouterotel.NewMiddleware())
			router.Use(NewCorsMiddleware())

			// Serve static files from local storage directory
			router.GET("/storage/*path", bunrouter.HTTPHandler(http.StripPrefix("/storage/", http.FileServer(http.Dir("storage")))))

			NewAuthClient(router)

			go func() {
				if err := GRPC("localhost:50050", func(grpc *grpc.Server) {
					proto.RegisterAuthServiceServer(grpc, NewAuthServer(app.dbHandler.Database(), NewAuthWrapper(app.config.Secret)))
				}); err != nil {
					panic(err)
				}
			}()

			// Application start --> start() callback
			go func() {
				if err := GRPC("localhost:50051", func(grpc *grpc.Server) {
					start(app.dbHandler.Database(), router, grpc)
				}); err != nil {
					panic(err)
				}
			}()
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
