# kartlobby
(WIP) A lobby server for [SRB2Kart](https://github.com/STJr/Kart-Public).

This application allows for more than a single dedicated server's worth of players to play on
the same server by leveraging containers to host multiple servers on the same port. When one dedicated server becomes full and a new player joins, a new dedicated server
is started for that player, up to a defined maximum number of rooms. When multiple rooms are active at once, new players will be added to the least-populated room first,
in order to balance players across all rooms.

## Progress
 - [x] Launching instances manually
 - [x] Away node PT_ASKINFO
 - [x] Joining active instances
 - [x] Launching new instances automatically
 - [ ] Closing dead instances
 - [ ] Configuration file
 - [ ] Load testing
 - [ ] TBD

## Usage
Run `pull_image.sh` to pull the latest Docker image. If the game ever updates, run it again to update the image to the latest version.

TODO: Maintain own Docker image rather than relying on [BrianAllred/srb2kart](https://github.com/BrianAllred/srb2kart).

## Acknowledgements
repo|for what
--|--
[Kart-Public](https://github.com/STJr/Kart-Public)|Kart.
[srb2kb](https://github.com/NielsjeNL/srb2kb)|Referenced for server info implementation.
[Colored server name [Tutorial] (+ chat text transparency)](https://mb.srb2.org/threads/colored-server-name-tutorial-chat-text-transparency.25474/)|Referenced for server name colors.
[BrianAllred/srb2kart](https://github.com/BrianAllred/srb2kart)|Used for Docker container images.
