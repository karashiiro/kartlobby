package main

type Configuration struct {
	GatewayPort int `default:"5029"`
	APIPort     int `default:"5030"`

	MaxRooms    int    `default:"1"`
	DockerImage string `default:"brianallred/srb2kart"`
}
