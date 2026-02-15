# Creating a new render postgres instance:

1) Create a new postgres service in render
2) Connect it to the visual studio postgres extension
    a) Add new connection, give it a name
    b) Fill in the follwoing fields: database, username, password and the hostname 
    which is the part between @ and / within the connection string (eg. hostname.oregon-postgres.render.com)
    c) Toggle use SSL
    d) Click connect
3) Initialize the db inside of the visual studio extension using db.sql
4) Update .env with the external connection string
5) Update the environment variable database_url of pivot_api in render with the internal connection string