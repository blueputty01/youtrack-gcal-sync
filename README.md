# YouTrack to Google Calendar Sync

This application synchronizes YouTrack issues with a Google Calendar. It is designed to run as a background service, periodically fetching updates from YouTrack and creating or updating corresponding events in your Google Calendar.

## Features

-   **Two-way Synchronization**: Syncs YouTrack issues to Google Calendar.
-   **OAuth 2.0 for Google**: Securely authenticates with the Google Calendar API using OAuth 2.0.
-   **Persistent State**: Uses a local SQLite database (`sync.db`) to keep track of synchronized items, preventing duplicate entries.
-   **Periodic Syncing**: Automatically runs synchronization at a configurable interval (default is 24 hours).
-   **Easy Configuration**: Uses a `config.json` file to manage all your settings.

## Requirements

-   Go 1.23 or later
-   A Google Cloud Platform project with the Google Calendar API enabled.
-   A YouTrack instance and a Permanent Token for API access.

## Setup & Configuration

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/youtrack-calendar-sync.git
    cd youtrack-calendar-sync
    ```

2.  **Create Google API Credentials:**
    -   Go to the [Google Cloud Console](https://console.cloud.google.com/).
    -   Create a new project or select an existing one.
    -   Enable the **Google Calendar API**.
    -   Go to "Credentials", click "Create Credentials", and choose "OAuth client ID".
    -   Select "Desktop app" as the application type.
    -   Copy the **Client ID** and **Client Secret**.

3.  **Create a `config.json` file:**
    Create a file named `config.json` in the root of the project directory with the following structure:

    ```json
    {
      "google_client_id": "YOUR_GOOGLE_CLIENT_ID",
      "google_client_secret": "YOUR_GOOGLE_CLIENT_SECRET",
      "google_redirect_url": "http://localhost:8080",
      "youtrack_base_url": "https://your-instance.myjetbrains.com/youtrack",
      "youtrack_permanent_token": "YOUR_YOUTRACK_PERMANENT_TOKEN",
      "youtrack_project_id": "YOUR_YOUTRACK_PROJECT_ID",
      "youtrack_query_project_id": "YOUR_YOUTRACK_QUERY",
      "google_calendar_id": "primary"
    }
    ```
    -   `google_calendar_id`: Use `"primary"` for the user's primary calendar, or the specific calendar ID.
    -   `youtrack_query`: The query to select issues from YouTrack (e.g., `#Resolved`).

4.  **Build the application:**
    ```bash
    go build
    ```

## Usage

1.  **Run the application:**
    ```bash
    ./youtrack-calendar-sync
    ```

2.  **Authorize with Google:**
    -   The first time you run the application, it will open a URL in your browser for Google authentication.
    -   Log in and grant the application permission to access your calendar.
    -   You will be redirected to a local URL. The application will capture the authorization token and save it as `token.json` for future use.

The application will then perform an initial synchronization and continue to sync periodically.

## How It Works

The application performs the following steps:
1.  Loads the configuration from `config.json`.
2.  Initializes the Google Calendar client. If a `token.json` file is not present, it initiates the OAuth 2.0 flow to get one.
3.  Initializes the YouTrack client using the provided base URL and permanent token.
4.  Sets up a local SQLite database (`sync.db`) to store mappings between YouTrack issues and Google Calendar events.
5.  The `Synchronizer` fetches issues from the specified YouTrack project based on your query.
6.  For each issue, it checks the local database to see if it has already been synced.
7.  If the issue is new or has been updated, it creates or updates an event in the specified Google Calendar.
8.  This process repeats at the interval defined by `syncInterval` in `main.go`.