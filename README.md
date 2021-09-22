# kartlobby
(WIP) A lobby server for [SRB2Kart](https://github.com/STJr/Kart-Public).

This application allows for more than a single dedicated server's worth of players to play on
the same server by leveraging containers to host multiple servers on the same port. When one dedicated server becomes full and a new player joins, a new dedicated server
is started for that player, up to a defined maximum number of rooms. When multiple rooms are active at once, new players will be added to the least-populated room first,
in order to balance players across all rooms.

*Discord lobby join requests are not guaranteed to work properly with this application.*

## Progress
 - [x] Launching instances manually
 - [x] Away node PT_ASKINFO
 - [x] Joining active instances
 - [x] Launching new instances automatically
 - [x] Closing dead instances
 - [x] Configuration file
 - [x] Sane colored server name support
 - [x] Add support for mounting config and addon volumes (Maybe? I can't tell if it mounted correctly.)
 - [x] Connection persistence through quick restarts ([#5](https://github.com/karashiiro/kartlobby/issues/5))
 - [ ] Waiting player handling ([#7](https://github.com/karashiiro/kartlobby/issues/7))
 - [ ] Ban list syncing ([#2](https://github.com/karashiiro/kartlobby/issues/2))
 - [ ] Load testing
 - [ ] Master Server support
 - [ ] TBD

## Usage
Requires Redis. Run `pull_image.sh` to pull the latest Docker image. If the game ever updates, run it again to update the image to the latest version.

## Configuration
The application comes preconfigured, but you can customize its configuration by creating a `config.yml` in the same directory as the program,
or by starting the application with `-config CONFIGPATH`, replacing `CONFIGPATH` with a path to your configuration file.

TODO: Documentation

## Acknowledgements
| repo                                                                                                                                                | for what                                   |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| [Kart-Public](https://github.com/STJr/Kart-Public)                                                                                                  | Kart.                                      |
| [srb2kb](https://github.com/NielsjeNL/srb2kb)                                                                                                       | Referenced for server info implementation. |
| [Colored server name [Tutorial] (+ chat text transparency)](https://mb.srb2.org/threads/colored-server-name-tutorial-chat-text-transparency.25474/) | Referenced for server name colors.         |
| [BrianAllred/srb2kart](https://github.com/BrianAllred/srb2kart)                                                                                     | Used for Docker container images.          |
