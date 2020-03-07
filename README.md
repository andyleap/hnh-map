# hnh-map

Automapper server for HnH, (mostly) compatible with https://github.com/APXEOLOG/hnh-auto-mapper-server

Docker image can be built from sources, or is available at https://hub.docker.com/r/andyleap/hnh-auto-mapper 
(automatically built by Docker's infrastructure from the github source)

Run it via whatever you feel like, it's listening internally on port 8080 and expects `/map` to be mounted as a volume 
(database and images are stored here), so something like the below will suffice.

    docker run -v /srv/hnh-map:/map -p 80:8080 andyleap/hnh-auto-mapper
  
Set it up under a domain name however you prefer (nginx reverse proxy, traefik, caddy, apache, whatever) and 
point your auto-mapping supported client at it (like Purus pasta)

Only other thing you need to do is setup users and set your zero grid.

Simply login as username admin, password admin, go to the admin portal, and hit "ADD USER".  Don't forget to toggle on all the roles (you'll need admin, at least)

Once you create your first user, you'll get kicked out and have to log in as it.
The admin user will be gone at this point.  Next you'll want to add users for anyone else, and then you'll need to create your tokens to upload stuff.

You'll probably want to set the prefix (this gets put at the front of the tokens, and should be something like `http://example.com`) to make it easier to configure clients.

The first client to connect will set the 0,0 grid, but you can wipe the data in the admin portal to reset (and the next client to connect should set a new 0,0 grid)

Roles
=====

Map: View the map
Upload: Send character, marker, and tile data to the server
Admin: modify server settings, create and edit users, wipe data
