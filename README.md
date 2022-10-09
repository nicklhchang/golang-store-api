Golang (Gorilla web toolkit) as an alternative to Node.js (Express.js) implementation. Still has server-side session authentication and protected routes. MongoDB still used for session and general purpose storage. 

More routes will be added over time.

If cloning, you will need an environment file explicitly called .env in this directory (./) with the field MONGO_URI. You will also require another environment file explicitly called .env in ./auth with the field SALT.

SALT=anyLongSequenceOfRunesYouWantItsJustBytesAfterAll

In the case of MONGO_URI, it will depend on the instance of MongoDB you want to connect to, because it's just a mongo connection URI.

https://www.mongodb.com/docs/manual/reference/connection-string/
