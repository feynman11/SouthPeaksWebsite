# South Peaks Cycling Club Website (Go + HTMX)

Official website for South Peaks Cycling Club (SPCC), built with Go and HTMX, hosted on Google App Engine.

## Tech Stack
- Backend: Go
- Frontend: HTML, CSS, HTMX
- Hosting: Google Cloud App Engine
- CI/CD: Google Cloud Build / GitHub Actions

## Local Development

1.  **Prerequisites:**
    *   Install Go (version 1.20 or higher recommended, matching `go.mod` and `app.yaml`).
    *   Git

2.  **Clone the repository:**
    ```bash
    git clone <your-repo-url>
    cd south-peaks-cycling-club
    ```

3.  **Run the Go application:**
    ```bash
    go mod tidy # To fetch dependencies if any were added
    go run main.go
    ```
    The website will be available at `http://localhost:8080`.

4.  **Customization:**
    *   Edit content and structure in `templates/index.html`.
    *   Modify styles in `static/style.css`.
    *   Update dynamic data (URLs, text) in `main.go` within the `TemplateData` struct or handlers.

## Deployment to Google Cloud App Engine

### Using Google Cloud Build (Triggered by GitHub Push)

1.  Ensure `cloudbuild.yaml` is configured.
2.  Ensure your Cloud Build trigger is set up to point to your repository and `cloudbuild.yaml`.
3.  Pushing to the configured branch (e.g., `main`) will automatically trigger a build and deployment.

### Using GitHub Actions

1.  Ensure `.github/workflows/deploy.yml` is configured.
2.  Add the required secrets to your GitHub repository settings:
    *   `GCP_PROJECT_ID`: Your Google Cloud Project ID.
    *   `GCP_SA_KEY`: Your Google Cloud service account key (JSON format, base64 encoded if required by the action, but `google-github-actions/setup-gcloud` usually handles JSON directly).
3.  Pushing to the `main` branch will trigger the workflow to build and deploy.

### Manual Deployment

1.  **Install Google Cloud SDK.**
2.  **Authenticate and set project:**
    ```bash
    gcloud auth login
    gcloud config set project YOUR_PROJECT_ID
    ```
3.  **Deploy:**
    From the project root directory:
    ```bash
    gcloud app deploy app.yaml
    ```
4.  **View your site:**
    ```bash
    gcloud app browse
    ```
