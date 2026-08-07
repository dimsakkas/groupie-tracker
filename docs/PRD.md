## 1. What Are We Building and Why

We are building **Groupie Tracker**, a web application that displays information about musical artists and bands using the Groupie Tracker API.

The website allows users to browse artists, view detailed information about each band, explore concert dates and locations, search across different artist attributes, filter results, and visualize concert locations on an interactive map.

The main challenge of this project is taking data from four separate API endpoints (`artists`, `locations`, `dates`, and `relation`) and combining them into one consistent application that is easy to navigate and understand.

Instead of treating the optional parts (Search, Filters, and Geolocation) as separate features added later, we plan to design the application around them from the beginning. This should keep the codebase cleaner and avoid rewriting parts of the project as new functionality is added.

Like the previous project, the goal is to build a stable web application with a clear structure, good error handling, and an interface that is simple for users to navigate.

## 2. How We Think About the Flow

When the user opens `http://localhost:8080`, the server receives a **GET** request and responds with the homepage. The homepage displays all available artists retrieved from the Groupie Tracker API.

From there, the user can browse the artist cards, search for a specific artist or member, apply filters, or select an artist to view more detailed information.

When an artist is selected, the browser sends another **GET** request to the corresponding artist page. The server retrieves the required information from the already loaded dataset and renders a page containing the artist's details, members, concert dates, concert locations, and an interactive map showing those locations.

One important design decision is that the API should only be fetched once when the server starts. Instead of requesting the API every time a user searches or opens a page, all data is loaded into memory and reused throughout the application's lifetime. This keeps the website fast and avoids unnecessary API requests.

Another decision we need to make early is how search and filters should work together. Instead of treating them as separate features, they should work on the same dataset. This allows users to search first and then filter the results (or vice versa) without duplicating logic in the backend.

That's the main flow of the application. Everything else—error handling, templates, search suggestions, filters, and geolocation—is built around making this experience smooth, reliable, and easy to use.

## 3. Features We Will Implement

The project requirements define the core functionality of the application. In addition to the mandatory features, we plan to implement the optional Search, Filters, and Geolocation quests as part of the main application rather than adding them at the end.

### Homepage

The homepage is the main entry point of the application.

It will display all available artists as cards containing basic information such as:

* Artist image
* Artist name
* Creation year
* First album date
* Number of members

Each card should be clickable and lead to the artist's dedicated page.

---

### Artist Details Page

Each artist will have a dedicated page displaying complete information about the selected artist.

The page will include:

* Artist image
* Artist name
* Members
* Creation year
* First album
* Concert locations
* Concert dates
* Interactive concert map

---

### Search

The application will include a search bar available from the homepage.

The search system should support:

* Artist/Band name
* Members
* Concert locations
* Creation date
* First album date

Search should be case-insensitive and provide live suggestions while the user is typing.

Each suggestion should identify what matched (Artist, Member, Location, etc.) before the user selects it.

---

### Filters

Users should be able to narrow the displayed artists using multiple filters.

The application will support:

* Creation year range
* First album year range
* Number of members
* Concert locations

Filters should work together and update the displayed artists without requiring the user to browse the entire dataset again.

---

### Geolocation

The artist page will include an interactive map displaying concert locations.

The application will convert concert locations into geographic coordinates and place markers on the map so users can visually explore where the artist has performed.

---

### Error Handling

The application should never crash because of invalid requests or missing data.

Custom error pages will be displayed whenever appropriate, including:

* 400 – Bad Request
* 404 – Not Found
* 500 – Internal Server Error

Every route should return the correct HTTP status code depending on the situation.

---

### Responsive Design

The website should remain usable on different screen sizes.

Layouts should adapt to desktop, tablet, and mobile devices while keeping navigation simple and information easy to read.

## 4. How We Split the Work

The goal is that **everyone contributes to both the frontend and backend** of the project. Each major feature should have one primary owner responsible for implementation and one reviewer responsible for testing, feedback, and code review before merging.

### Phase 1 — Planning (everyone together)

* Agree on the project structure before writing any code.
* Define the data models that will be shared across the application.
* Decide how the homepage and artist pages should be organized.
* Set up the repository and confirm everyone can run the server locally.

### Phase 2 — Core Features

| Feature                    | First Owner | Reviewer |
| -------------------------- | ----------- | -------- |
| API fetching & data models |             |          |
| Homepage & artist cards    |             |          |
| Artist details page        |             |          |
| HTML templates & CSS       |             |          |

### Phase 3 — Optional Features

| Feature                       | First Owner | Reviewer |
| ----------------------------- | ----------- | -------- |
| Search system                 |             |          |
| Filters                       |             |          |
| Geolocation & map             |             |          |
| Error pages & status handling |             |          |

### Phase 4 — Second Pass

* Each team member works on a feature they did **not** implement during the first pass.
* Review the code, improve it if needed, and write tests where appropriate.
* No feature should be merged without at least one other team member reviewing it.

### Phase 5 — Testing & Audit Preparation

* Test every route and major feature as a team.
* Verify all project requirements are satisfied.
* Review error handling and HTTP status codes.
* Finalize the README and project documentation together.

## 5. Endpoints

| Method | Route          | Description                                           | Success | Error           |
| ------ | -------------- | ----------------------------------------------------- | ------- | --------------- |
| GET    | `/`            | Serves the homepage displaying all artists            | 200     | 500             |
| GET    | `/artist/{id}` | Displays detailed information for the selected artist | 200     | 400 / 404 / 500 |
| GET    | `/search`      | Returns search suggestions or search results          | 200     | 400 / 500       |
| GET    | `/filter`      | Returns artists matching the selected filters         | 200     | 400 / 500       |
| GET    | `/about`       | About page (team, project description) *(optional)*   | 200     | 404             |
| ANY    | `/*`           | Catch-all for unknown routes                          | —       | 404             |

### Notes

* The homepage (`/`) loads all artists and displays them as cards.
* `/artist/{id}` receives an artist ID and renders the corresponding artist page.
* `/search` is intended for the live search bar and returns matching results while the user types.
* `/filter` returns artists matching the selected filter criteria.
* Unknown routes should always return a custom **404 Not Found** page.

## 6. API Endpoints

The application retrieves its data from the Groupie Tracker API, which is divided into four endpoints. Each endpoint provides a different part of the dataset and must be combined to build the final application model.

| Endpoint     | Purpose                                                                                                   |
| ------------ | --------------------------------------------------------------------------------------------------------- |
| `/artists`   | Returns general information about each artist or band (name, image, creation date, first album, members). |
| `/locations` | Returns all concert locations for each artist.                                                            |
| `/dates`     | Returns all concert dates for each artist.                                                                |
| `/relation`  | Links artists, locations, and dates together.                                                             |

### Notes

* All API data should be fetched when the server starts.
* The application should reuse the loaded data instead of requesting the API for every user action.
* The four endpoints should be combined into one consistent data model before being used by the rest of the application.
* If the API cannot be reached or returns invalid data, the application should display an appropriate error page instead of crashing.

## 7. Data Models

The application will combine the four API endpoints into a single data model that can be used throughout the project.

### Artist

The main model represents an artist or band and contains:

* ID
* Name
* Image
* Creation Year
* First Album
* Members
* Concert Locations
* Concert Dates

This model is used by the homepage, artist details page, search system, and filters.

---

### Search Result

Search suggestions should contain enough information for the frontend to display meaningful results.

Fields:

* Matched Value
* Match Type (Artist, Member, Location, etc.)
* Artist ID

This allows the frontend to identify what matched and redirect the user to the correct artist page.

---

### Filter Options

The filter system will work with the following values:

* Creation Year Range
* First Album Year Range
* Number of Members
* Concert Locations

Multiple filters should be applied at the same time to narrow the displayed results.

---

### Geolocation

Each concert location displayed on the map should contain:

* Location Name
* Latitude
* Longitude

These coordinates will be used to place markers on the interactive map.

## 8. HTTP Status Codes & Error Handling

The application should always respond with the appropriate HTTP status code. Unexpected situations should never cause the server to crash, and users should receive a meaningful error page instead of a generic browser error.

| Status Code                   | Description                              | Example                                                                       |
| ----------------------------- | ---------------------------------------- | ----------------------------------------------------------------------------- |
| **200 OK**                    | Request completed successfully.          | Homepage, artist page, search results.                                        |
| **400 Bad Request**           | Invalid request or malformed parameters. | Invalid artist ID or incorrect query parameters.                              |
| **404 Not Found**             | Requested resource does not exist.       | Unknown route or artist ID not found.                                         |
| **500 Internal Server Error** | Unexpected server-side error.            | API unavailable, template execution failure, or unexpected application error. |

### Error Handling Guidelines

* All handlers should validate user input before processing requests.
* Invalid routes should return a custom **404 Not Found** page.
* Invalid query parameters should return **400 Bad Request**.
* Internal application errors should return **500 Internal Server Error**.
* Errors should be logged on the server for debugging, while users should only see a clear and user-friendly error page.
* The application should continue running even if an individual request fails.

## 9. Repository Structure

The project follows a modular structure that separates the backend, frontend, templates, and static assets. This organization keeps the codebase easy to navigate and maintain.

```text
groupie-tracker/
│
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
│
├── handlers/                    # HTTP handlers
│   ├── home.go
│   ├── artist.go
│   ├── search.go
│   ├── filter.go
│   ├── errors.go
│   └── handlers_test.go
│
├── api/                         # API communication
│   ├── fetch.go
│   ├── parser.go
│   └── api_test.go
│
├── models/                      # Shared application models
│   ├── artist.go
│   ├── search.go
│   └── models_test.go
│
├── utils/                       # Helper functions
│   ├── geocode.go
│   ├── helpers.go
│   └── utils_test.go
│
├── templates/                   # HTML templates
│   ├── index.html
│   ├── artist.html
│   ├── 404.html
│   ├── 500.html
│   └── about.html               # Optional
│
├── static/
│   ├── css/
│   │   ├── style.css
│   │   └── responsive.css
│   │
│   ├── js/
│   │   ├── search.js
│   │   ├── filters.js
│   │   └── map.js
│   │
│   └── images/
│
├── tests/                       # Integration or shared tests
│
├── README.md
├── go.mod
└── go.sum
```

### Directory Overview

| Directory    | Purpose                                                    |
| ------------ | ---------------------------------------------------------- |
| `cmd/server` | Starts the application and configures the HTTP server.     |
| `handlers`   | Processes incoming HTTP requests and renders responses.    |
| `api`        | Retrieves and processes data from the Groupie Tracker API. |
| `models`     | Shared data structures used throughout the application.    |
| `utils`      | Helper functions used across multiple packages.            |
| `templates`  | HTML templates rendered by the backend.                    |
| `static`     | CSS, JavaScript, images, and other static assets.          |
| `tests`      | Integration tests and shared testing utilities.            |

### Notes

* Each package should have a single responsibility.
* Business logic should remain outside the HTTP handlers whenever possible.
* Shared models and helper functions should not be duplicated.
* The structure may evolve during development, but responsibilities should remain clearly separated.

## 10. Usage

### Starting the Application

Run the server:

```bash
go run ./cmd/server
```

By default, the application will be available at:

```text
http://localhost:8080
```

---

### Basic User Flow

1. Open the homepage.
2. Browse the list of available artists.
3. Use the search bar or filters to find specific artists.
4. Select an artist to open their details page.
5. View artist information, concert dates, concert locations, and the interactive map.
6. Return to the homepage and continue exploring.

---

### Expected Behavior

* The application should load all artist data before serving requests.
* Search suggestions should appear while the user types.
* Filters should update the displayed artists based on the selected criteria.
* Every artist page should display complete information retrieved from the API.
* Invalid routes or requests should display the appropriate error page instead of crashing the server.

## 11. Out of Scope

The following features are not planned for this project and will not be implemented unless the project requirements change:

* User authentication or user accounts.
* Databases or persistent data storage.
* Admin panel or content management system.
* Editing, creating, or deleting artist information.
* Saving favorite artists or personalized playlists.
* User comments or reviews.
* Real-time concert updates.
* Payment or ticket purchasing functionality.
* Any functionality unrelated to displaying and interacting with the Groupie Tracker dataset.

The project focuses solely on retrieving, processing, and displaying information provided by the Groupie Tracker API while implementing the required Search, Filters, and Geolocation features.

## 12. Acceptance Criteria

The project will be considered complete when the following requirements have been successfully implemented and tested.

### Core Functionality

* [ ] The homepage loads successfully.
* [ ] All artists are displayed correctly.
* [ ] Each artist card links to its corresponding details page.
* [ ] Artist pages display all required information.
* [ ] Concert locations and dates are displayed correctly.

---

### Search

* [ ] Search works for artist/band names.
* [ ] Search works for members.
* [ ] Search works for concert locations.
* [ ] Search works for creation dates.
* [ ] Search works for first album dates.
* [ ] Search is case-insensitive.
* [ ] Search suggestions appear while typing.
* [ ] Every suggestion identifies its type (Artist, Member, Location, etc.).

---

### Filters

* [ ] Artists can be filtered by creation year.
* [ ] Artists can be filtered by first album year.
* [ ] Artists can be filtered by number of members.
* [ ] Artists can be filtered by concert locations.
* [ ] Multiple filters can be applied simultaneously.

---

### Geolocation

* [ ] Concert locations are converted into geographic coordinates.
* [ ] Artist pages display an interactive map.
* [ ] Concert locations are shown as markers on the map.

---

### Error Handling

* [ ] Invalid routes return a custom 404 page.
* [ ] Invalid requests return the appropriate HTTP status code.
* [ ] Internal errors are handled without crashing the server.

---

### User Interface

* [ ] The website is easy to navigate.
* [ ] The layout works correctly on different screen sizes.
* [ ] Navigation between pages is intuitive.

---

### Code Quality

* [ ] The backend is written entirely in Go using the standard library.
* [ ] The project follows a clean and organized structure.
* [ ] Code is readable and documented where necessary.
* [ ] Unit tests are included for important components where appropriate.

---

### Final Review

* [ ] All mandatory project requirements have been implemented.
* [ ] All planned optional features (Search, Filters, and Geolocation) are implemented.
* [ ] The application behaves as expected during manual testing.
* [ ] The project is ready for the audit.
