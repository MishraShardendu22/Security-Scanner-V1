StackGuard Assignment: Hugging Face
Objective
Design and implement an API-based system that scans Hugging Face-hosted assets (models, datasets, and spaces) for potential secrets and tokens using regex-based pattern matching.
The system should produce structured JSON results that can later integrate with StackGuard’s secret scanning binary.

Problem Statement
StackGuard wants to extend its secret scanning capabilities to AI model ecosystems.
 You need to build a backend API that:
Fetches files from Hugging Face resources (Model / Dataset / Space).
Scans their content using regex-based secret detectors.
Returns structured findings via API responses.
Optionally persists and contextualizes results for visualization.



API Requirements
1. Scan Endpoint
POST /api/scan
Request Body Example:
{
  "model_id": "microsoft/phi-3",
  "dataset_id": null,
  "space_id": null,
  "org": "microsoft",
  "user": null,
  "include_discussions": true,
  "include_prs": false
}

Expected Behavior:
Use Hugging Face Hub API to fetch files, discussions, and PRs for the provided IDs.
Run regex-based scanning for secrets (e.g. ghp_, AKIA, AIza, slack-bot-token, etc.).
Return structured JSON results like:


{
  "scan_id": "SG-2025-1012-001",
  "scanned_resources": [
    {
      "type": "model",
      "id": "microsoft/phi-3",
      "findings": [
        {
          "secret_type": "AWS Access Key",
          "pattern": "AKIA********",
          "file": "config.json",
          "line": 24
        }
      ]
    }
  ],
  "timestamp": "2025-10-12T12:30:00Z"
}


2. Results Storage Endpoint
POST /api/store
Body: JSON output of the scan
 Functionality:
Store the scan results in a local database (SQLite / Mongo / Postgres).
Add metadata: context, scan_source, detected_at.


Return confirmation:

 { "status": "stored", "scan_id": "SG-2025-1012-001" }

3. Fetch Results Endpoint
GET /api/results/{scan_id}
Returns stored scan details and contextual metadata
Implementation Notes
Backend language: Go or Python (FastAPI/Flask) preferred.
Use Hugging Face Hub API to fetch resources.
Use 10–20 regex patterns for secret detection.
Output must be JSON structured, ready for later integration with StackGuard’s binary.
Add error handling, rate limiting, and logging where appropriate.



Showcase
A lightweight dashboard endpoint (/api/dashboard) to show all stored results grouped by resource type and severity.
Add organization-level scanning support — fetch all models/datasets/spaces under a given org/user.



Timeframe
Submission Deadline: 2–3 days
Submit your GitHub repository link with a short README explaining setup and approach.

