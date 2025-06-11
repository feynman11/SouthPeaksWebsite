# South Peaks Cycling Club Website (Go + HTMX)

Official website for South Peaks Cycling Club (SPCC), based in Borrowash, Derbyshire, UK. This application is built with Go, uses HTMX for dynamic content, and manages members data in MongoDB.

## Features

*   **Responsive Design:** Optimized for various devices.
*   **Club Branding:** Integrated logo and themed colors (black, dark red, white) from SPCC branding.
*   **External Links:** Direct links to Strava and Instagram.
*   **Club Information:** Details about regular rides and club philosophy.
*   **Strava Club Widget:** Embedded live widget displaying latest club rides.
*   **User Authentication:** Secure login via Strava OAuth 2.0.
*   **Members Area:** A restricted page for logged-in club members.
*   **Member Management (Admin):** Admins can toggle "paid member" status for users.
*   **Data Storage:** Member data stored in MongoDB.
*   **Deployment:** Automated CI/CD using Google Cloud Build / GitHub Actions.
*   **Fast Hosting:** Hosted on Google Cloud App Engine (or Cloud Run, depending on your final deployment target).

## Tech Stack

*   **Backend:** Go
*   **Frontend:** HTML, CSS, HTMX
*   **Database:** MongoDB
*   **OAuth:** Strava API
*   **Hosting:** Google Cloud App Engine / Cloud Run
*   **CI/CD:** Google Cloud Build / GitHub Actions

## Local Development Setup

To run the application locally, you'll need Go, Git, and a running MongoDB instance (local or cloud, e.g. MongoDB Atlas).

### Prerequisites

*   [Go](https://golang.org/doc/install) (version 1.20+ recommended)
*   [Git](https://git-scm.com/downloads)
*   [MongoDB](https://www.mongodb.com/try/download/community) (local or [MongoDB Atlas](https://www.mongodb.com/atlas/database))
*   (Optional) [MongoDB Compass](https://www.mongodb.com/products/compass) for GUI management

### Steps

1.  **Clone the repository:**
    ```bash
    git clone <your-repo-url>
    cd south-peaks-cycling-club
    ```

2.  **Initialize Go modules and fetch dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Place Static Assets:**
    *   Ensure your club logo (`spcc_logo.jpg`) is in the `static/` directory.
    *   Generate a favicon (e.g., `favicon.png`) from your logo and place it in the `static/` directory.

4.  **Configure Strava API Application:**
    *   Go to [Strava Developer Portal](https://developers.strava.com/docs/).
    *   Create a new API Application.
    *   Set the **Authorization Callback Domain** to `localhost` for local development.
    *   Note your **Client ID** and **Client Secret**.

5.  **Set up MongoDB:**
    *   **Local:** Install and start MongoDB, or
    *   **Cloud:** Create a free cluster on [MongoDB Atlas](https://www.mongodb.com/atlas/database).
    *   **Connection String:** Obtain your MongoDB URI (e.g., `mongodb://localhost:27017/southpeakscc` or from Atlas dashboard).

6.  **Run the Go application:**
    Open a **new terminal tab/window** in your project root. Set the required environment variables:

    ```bash
    # IMPORTANT: Update with your actual Strava credentials and local Go app URL
    export STRAVA_CLIENT_ID="YOUR_STRAVA_CLIENT_ID"
    export STRAVA_CLIENT_SECRET="YOUR_STRAVA_CLIENT_SECRET"
    export SESSION_SECRET_KEY="A_VERY_LONG_RANDOM_STRING_FOR_SESSIONS" # e.g., openssl rand -base64 32
    export OAUTH_CALLBACK_URL="http://localhost:8081" # Must match your Go app's port

    # MongoDB connection string
    export MONGODB_URI="mongodb://localhost:27017/southpeakscc" # Or your Atlas URI

    # Run Go app on a port of your choice
    export PORT="8081"

    go run .
    ```
    The website will be available at `http://localhost:8081`.

7.  **Verify Admin Status:** After you log in for the first time via Strava, manually update your user document in MongoDB to set `isAdmin: true` if you want to test admin functionality.

## Deployment to Google Cloud

This application is designed for deployment on Google Cloud App Engine or Cloud Run. Ensure your chosen service account has roles like `App Engine Admin`, `Cloud Datastore User`, `Service Account User`, and `Storage Admin`.  
**Note:** You must also provide your MongoDB connection string as a secret or environment variable in your deployment configuration.