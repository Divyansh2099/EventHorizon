package eventhorizon

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/eventhorizon/pkg/connection"
	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/router"
	"github.com/eventhorizon/pkg/server"
	"github.com/eventhorizon/pkg/tls"
)

// App is the main EventHorizon application structure, wrapping the low-level RIO server.
type App struct {
	srv *server.Server
}

// New initializes a new EventHorizon App instance.
func New(port int) *App {
	return &App{
		srv: server.NewServer(port),
	}
}

// GET registers a new GET route.
func (app *App) GET(path string, handler func(c *Context)) {
	app.srv.Router.Handle("GET", path, wrapHandler(handler))
}

// POST registers a new POST route.
func (app *App) POST(path string, handler func(c *Context)) {
	app.srv.Router.Handle("POST", path, wrapHandler(handler))
}

// WS registers a global WebSocket frame handler.
// Note: Since this is global per the current architecture, path isn't used internally yet.
func (app *App) WS(path string, handler func(c *Context, frame []byte)) {
	app.srv.Router.WSRoute = func(connPtr any, framePtr any) {
		conn := connPtr.(*connection.Conn)
		f := framePtr.(parser.WSFrame)
		
		ctx := &Context{
			Conn: conn,
			// WS frames don't have a RequestCtx in the current loop, but we can pass nil or mock
		}
		handler(ctx, f.Payload)
	}
}

// wrapHandler converts the public Context signature into the internal RequestCtx signature.
func wrapHandler(h func(c *Context)) router.HandlerFunc {
	return func(ctx *parser.RequestCtx) {
		c := &Context{
			Req:  ctx,
			Conn: (*connection.Conn)(ctx.Conn),
		}
		h(c)
	}
}

// Listen starts the server using the provided TLS certificate and key.
func (app *App) Listen(certFile, password string) error {
	log.Printf("Starting EventHorizon API...")
	
	if err := tls.InitSchannel(certFile, password); err != nil {
		return err
	}

	if err := app.srv.Start(); err != nil {
		return err
	}

	// Wait for interrupt signal to gracefully shut down the server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down EventHorizon API...")
	app.Stop()
	return nil
}

// Stop gracefully shuts down the server.
func (app *App) Stop() {
	app.srv.Stop()
}
