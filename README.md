# Technologies: 

**Mise** – Version manager (Node and Go).  
**Capacitor** - Packages the frontend as a native mobile app for iOS and Android.  

## Frontend
**npm** – Initially used to add Tailwind; now also manages Serve and Concurrent.  
**Tailwind** – Quickly style components with utility classes.  
**Serve** – Serves the public folder for development, so HTMX and Tailwind CSS work correctly with index.html.  
**Concurrent** – Runs multiple commands at once to watch CSS, HTML, and Go files.  
**Alpine** - Enables a Single Page Application experience.  

## Backend
**Go** – Chosen for simplicity and easy API development.  
**Air** – Watches Go files and reloads during development.  

# TODO: 

- Improve API URL switching for production and development?
- Install Alpine with npm and then add bundler like vite?
- Save logged in user? Sessions? Jwt?
- Failed login message? Loading button with feedback?
- Component.js must automatically create the components in the directory
- Figure out pivot status initialization
- Save credentials
- local storage for last selected pivot
- loading 
- merge pivot and pivot_status tables