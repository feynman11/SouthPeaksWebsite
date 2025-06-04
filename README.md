# South Peaks Cycling Club Website (Go + HTMX)

Official website for South Peaks Cycling Club (SPCC), based in Borrowash, Derbyshire, UK. This application is built with Go, uses HTMX for dynamic content, and manages members data in Google Cloud Firestore.

## Features

*   **Responsive Design:** Optimized for various devices.
*   **Club Branding:** Integrated logo and themed colors (black, dark red, white) from SPCC branding.
*   **External Links:** Direct links to Strava and Instagram.
*   **Club Information:** Details about regular rides and club philosophy.
*   **Strava Club Widget:** Embedded live widget displaying latest club rides.
*   **User Authentication:** Secure login via Strava OAuth 2.0.
*   **Members Area:** A restricted page for logged-in club members.
*   **Member Management (Admin):** Admins can toggle "paid member" status for users.
*   **Data Storage:** Member data stored in Google Cloud Firestore.
*   **Deployment:** Automated CI/CD using Google Cloud Build / GitHub Actions.
*   **Fast Hosting:** Hosted on Google Cloud App Engine (or Cloud Run, depending on your final deployment target).

## Tech Stack

*   **Backend:** Go
*   **Frontend:** HTML, CSS, HTMX
*   **Database:** Google Cloud Firestore
*   **OAuth:** Strava API
*   **Hosting:** Google Cloud App Engine / Cloud Run
*   **CI/CD:** Google Cloud Build / GitHub Actions

## Local Development Setup

To run the application locally, you'll need Go, Git, `firebase-tools` (for the Firestore emulator), and Google Cloud SDK for authentication.

### Prerequisites

*   [Go](https://golang.org/doc/install) (version 1.20+ recommended)
*   [Git](https://git-scm.com/downloads)
*   [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) (for `gcloud` and `firebase` commands)
*   [Node.js and npm](https://nodejs.org/en/download/) (required for `firebase-tools`)
*   [Firebase CLI](https://firebase.google.com/docs/cli#install_the_firebase_cli) (via npm): `npm install -g firebase-tools`

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

5.  **Set up Google Cloud Project (Firestore):**
    *   **Enable Firestore:** Go to [Firestore in Google Cloud Console](https://console.cloud.google.com/datastore/setup?project=YOUR_PROJECT_ID). If not already set up, choose "Native mode" and select a region (e.g., `europe-west1`).
    *   **Note your Firestore Database ID:** If you created a named database (e.g., `southpeaksdb`), ensure the `firestoreDatabaseID` constant in `main.go` matches. If you used the default, remove that constant.
    *   **Authentication (for local access to live Firestore if not using emulator):** `gcloud auth application-default login`

6.  **Run Firestore Emulator (Recommended for Local Development):**
    *   Initialize Firebase in your project root (if not already done): `firebase init` (select Firestore, use existing project, accept defaults).
    *   Start the emulator (it usually runs on `localhost:8080`):
        ```bash
        firebase emulators:start --only firestore
        ```
        (You can view the emulator UI at `http://localhost:4000` by default).

7.  **Run the Go application:**
    Open a **new terminal tab/window** (separate from the emulator) in your project root. Set the required environment variables:

    ```bash
    # IMPORTANT: Update with your actual Strava credentials and local Go app URL
    export STRAVA_CLIENT_ID="YOUR_STRAVA_CLIENT_ID"
    export STRAVA_CLIENT_SECRET="YOUR_STRAVA_CLIENT_SECRET"
    export SESSION_SECRET_KEY="A_VERY_LONG_RANDOM_STRING_FOR_SESSIONS" # e.g., openssl rand -base64 32
    export OAUTH_CALLBACK_URL="http://localhost:8081" # Must match your Go app's port

    # Point Go app to the local Firestore emulator (if running)
    export FIRESTORE_EMULATOR_HOST="localhost:8080" # Match the port Firebase emulator started on

    # Run Go app on a different port to avoid conflict with emulator
    export PORT="8081"

    go run .
    ```
    The website will be available at `http://localhost:8081`.

8.  **Verify Admin Status:** After you log in for the first time via Strava, manually update your user document in the Firestore Console (`https://console.cloud.google.com/firestore/data/YOUR_DATABASE_ID/users/YOUR_STRAVA_ID`) to set `isAdmin: true` if you want to test admin functionality.

## Deployment to Google Cloud

This application is designed for deployment on Google Cloud App Engine or Cloud Run. Ensure your chosen service account has roles like `App Engine Admin`, `Cloud Datastore User`, `Service Account User`, and `Storage Admin`.

### Using Google Cloud Build (Triggered by GitHub Push)

1.  **Enable APIs & Set Permissions:**
    ```bash
    gcloud services enable cloudbuild.googleapis.com appengine.googleapis.com storage.googleapis.com
    # Grant Cloud Build SA roles (replace PROJECT_NUMBER and PROJECT_ID)
    PROJECT_NUMBER=$(gcloud projects describe YOUR_PROJECT_ID --format="value(projectNumber)")
    gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
        --member="serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com" \
        --role="roles/appengine.appAdmin"
    gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
        --member="serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com" \
        --role="roles/iam.serviceAccountUser"
    gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
        --member="serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com" \
        --role="roles/datastore.user" # Or roles/datastore.owner
    gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
        --member="serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com" \
        --role="roles/storage.admin"
    ```

2.  **Configure Environment Variables in Cloud Build:**
    You must set your secrets securely. In the Cloud Build trigger settings (Google Cloud Console > Cloud Build > Triggers > Your Trigger > Edit), under "Build variables", add:
    *   `_STRAVA_CLIENT_ID`: Your Strava Client ID
    *   `_STRAVA_CLIENT_SECRET`: Your Strava Client Secret
    *   `_SESSION_SECRET_KEY`: Your session secret key
    *   `_OAUTH_CALLBACK_URL`: The **HTTPS URL of your deployed application** (e.g., `https://www.southpeakscc.co.uk` or `https://YOUR_APP_ID.REGION.run.app`)

    Then, modify your `cloudbuild.yaml` to use these variables in the build step, perhaps via `env_variables` in a build step or a `Dockerfile`. A more secure way is to use Google Secret Manager and reference secrets in `cloudbuild.yaml`.

3.  **Create automatic trigger:**
    Go to Google Cloud Console > Cloud Build > Triggers. Create a trigger connected to your GitHub repository, set to trigger on pushes to `main` (or your primary branch), using `cloudbuild.yaml` as the build configuration file.