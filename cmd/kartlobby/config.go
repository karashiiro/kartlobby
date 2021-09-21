package main

type Configuration struct {
	GatewayPort  int    `default:"5029"`
	APIPort      int    `default:"5030"`
	RedisAddress string `default:"localhost:6379"`

	// The cache key used for the game instance manager, if you run
	// multiple lobbies on the same physical server alongside Redis.
	GIMKey string `default:"kartlobby_gameinstancemanager"`

	Motd        string
	MaxRooms    int    `default:"1"`
	DockerImage string `default:"brianallred/srb2kart"`

	GameConfig string
	GameAddons string
}
