# Google Maps Routing Engine for OSMAnd

## Summary

A routing engine for OSMAnd using the Google Maps Routes API; built on Go.

The first prototype was built on Python using FastAPI. It worked perfectly, but the load time when deployed on as a serverless service (Cloud Run, Lambda) was too slow for seamless use. The load time of the Go container is consistently under one second. OSMAnd is able to fetch and process the routes and display the navigation details in about three seconds.


## Instructions

Deploy main.go with the following environment variables:

- **GOOGLE_MAPS_API_KEY**: A GCP API Key with permission to use the Google Maps Routing APIs.
- **ROUTER_APP_AUTH_TOKEN**: An authentication token to use for accessing the router app. 

Configure OSMAnd to use the GPX mode and use the URL in the following format 
https://<domain_name>/route/<travel_mode>/<ROUTER_APP_AUTH_TOKEN>

Here, domain_name is whatever you are using for hosting your app. Travel mode are commonly "car", "motorcycle", "scooter", "bike", "walking", "transit", etc.


## Roadmap

* Use OSRM protocol instead of GPX.
* Allow usage of multiple waypoints (more than 3)
* Proper representation of "legs" of the route.

