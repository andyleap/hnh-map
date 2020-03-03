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

By default, the only user is an "admin only" with username and password equal to "admin"
Create your own admin account like so:

    curl --user admin:admin "http://localhost/api/admin/setUser?user=vendan&pass=hnh&auths=map,admin"
    
I'm using localhost in these examples, but swap out for your domain (and port, if applicable), of course.  
Once that's done, you can add additional "mapping only" users by chopping the `,admin` bit off.
Or create additional admin users to give them the ability to add other people as well...

Finally, you'll need to set the zero grid, like so:

    curl --user vendan:hnh "http://localhost/api/admin/setZeroGrid?gridId=<GridID>"
    
Then I'd suggest restarting your client, and everything should start working!
