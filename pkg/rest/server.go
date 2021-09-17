package rest

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type Handler = func() (interface{}, error)

type RESTServerOptions struct {
	Port int
}

type RESTServer struct {
	port   int
	server *fiber.App
}

func NewServer(opts *RESTServerOptions) *RESTServer {
	r := RESTServer{
		port:   opts.Port,
		server: fiber.New(),
	}

	return &r
}

func (r *RESTServer) Run() error {
	return r.server.Listen(":" + fmt.Sprint(r.port))
}

func (r *RESTServer) Get(route string, fn Handler) {
	r.server.Get(route, func(c *fiber.Ctx) error {
		o, err := fn()
		if err != nil {
			return err
		}

		c.Context().SetContentType("application/json")
		data, err := json.Marshal(o)
		if err != nil {
			return err
		}

		return c.Send([]byte(data))
	})
}
