\# Google Maps Routing Engine for OSMAnd



A routing engine for OSMAnd using the Google Maps Routing API built on Go.



The first prototype was built on Python using FastAPI. It worked perfectly, but the load time when deployed on as a serverless service (Cloud Run, Lambda) was too slow for seamless use. The load time of the Go container is consistently under one second. OSMAnd is able to fetch and process the routes and display the navigation details in about three seconds.





\## Roadmap



* Use OSRM protocol instead of GPX.
* Allow usage of multiple waypoints (more than 3)
* Proper representation of "legs" of the route.

