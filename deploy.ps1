# ============================================================
# StreamMaxing v3 - Full Deployment Script
# ============================================================
# Usage:
#   .\deploy.ps1              - Deploy everything (backend + frontend)
#   .\deploy.ps1 -BackendOnly - Deploy only the Lambda backend
#   .\deploy.ps1 -FrontendOnly - Deploy only the frontend to S3 + invalidate CloudFront
# ============================================================

param(
    [switch]$BackendOnly,
    [switch]$FrontendOnly
)

$ErrorActionPreference = "Stop"

# --- Load environment variables from .env ---
function Load-EnvFile {
    param([string]$Path)
    $vars = @{}
    if (Test-Path $Path) {
        Get-Content $Path | ForEach-Object {
            $line = $_.Trim()
            if ($line -and -not $line.StartsWith("#")) {
                $parts = $line -split "=", 2
                if ($parts.Count -eq 2) {
                    $vars[$parts[0].Trim()] = $parts[1].Trim()
                }
            }
        }
    }
    return $vars
}

# --- Preflight checks ---
Write-Host "`n=== StreamMaxing v3 Deployment ===" -ForegroundColor Cyan

# Check AWS CLI
if (-not (Get-Command aws -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: AWS CLI not found. Install from https://aws.amazon.com/cli/" -ForegroundColor Red
    exit 1
}

# Check SAM CLI
if (-not (Get-Command sam -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: AWS SAM CLI not found. Install from https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html" -ForegroundColor Red
    exit 1
}

# Check Go
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: Go not found. Install from https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

# Check Node.js
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: Node.js not found. Install from https://nodejs.org/" -ForegroundColor Red
    exit 1
}

# Load secrets from .env
$envVars = Load-EnvFile -Path "$PSScriptRoot\.env"
$requiredKeys = @(
    "DATABASE_URL", "DISCORD_CLIENT_ID", "DISCORD_CLIENT_SECRET",
    "DISCORD_APPLICATION_ID", "DISCORD_PUBLIC_KEY", "DISCORD_BOT_TOKEN",
    "TWITCH_CLIENT_ID", "TWITCH_CLIENT_SECRET", "TWITCH_WEBHOOK_SECRET", "JWT_SECRET"
)
foreach ($key in $requiredKeys) {
    if (-not $envVars[$key]) {
        Write-Host "ERROR: Missing $key in .env file" -ForegroundColor Red
        exit 1
    }
}

Write-Host "Environment loaded from .env" -ForegroundColor Green

# ---------------------------------------------
# BACKEND DEPLOYMENT
# ---------------------------------------------
if (-not $FrontendOnly) {
    Write-Host "`n--- Building Backend (Go -> Linux x86_64) ---" -ForegroundColor Yellow

    # Create build output directory
    New-Item -ItemType Directory -Force -Path "$PSScriptRoot\build\lambda" | Out-Null

    # Cross-compile for Lambda
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"

    Push-Location "$PSScriptRoot\backend"
    go build -tags lambda.norpc -o "$PSScriptRoot\build\lambda\bootstrap" ./cmd/lambda/main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Go build failed" -ForegroundColor Red
        Pop-Location
        exit 1
    }
    Pop-Location

    # Reset env
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

    Write-Host "Backend binary built: build/lambda/bootstrap" -ForegroundColor Green

    # Deploy with SAM
    Write-Host "`n--- Deploying Infrastructure (SAM) ---" -ForegroundColor Yellow

    # Check if stack already exists to get the CloudFront URL for SiteUrl param
    $siteUrl = ""
    $stackExists = $false
    try {
        $existingOutputs = aws cloudformation describe-stacks `
            --stack-name streammaxing `
            --region us-west-2 `
            --query "Stacks[0].Outputs" `
            --output json 2>$null | ConvertFrom-Json
        if ($existingOutputs) {
            $stackExists = $true
            $siteUrl = ($existingOutputs | Where-Object { $_.OutputKey -eq "CloudFrontUrl" }).OutputValue
            Write-Host "  Existing stack found. SiteUrl: $siteUrl" -ForegroundColor DarkGray
        }
    } catch {
        # Stack doesn't exist yet - first deploy
    }

    $paramOverrides = @(
        "DatabaseUrl='$($envVars['DATABASE_URL'])'"
        "DiscordClientId='$($envVars['DISCORD_CLIENT_ID'])'"
        "DiscordClientSecret='$($envVars['DISCORD_CLIENT_SECRET'])'"
        "DiscordApplicationId='$($envVars['DISCORD_APPLICATION_ID'])'"
        "DiscordPublicKey='$($envVars['DISCORD_PUBLIC_KEY'])'"
        "DiscordBotToken='$($envVars['DISCORD_BOT_TOKEN'])'"
        "TwitchClientId='$($envVars['TWITCH_CLIENT_ID'])'"
        "TwitchClientSecret='$($envVars['TWITCH_CLIENT_SECRET'])'"
        "TwitchWebhookSecret='$($envVars['TWITCH_WEBHOOK_SECRET'])'"
        "JwtSecret='$($envVars['JWT_SECRET'])'"
        "SiteUrl='$siteUrl'"
    )

    sam deploy `
        --template-file "$PSScriptRoot\template.yaml" `
        --stack-name streammaxing `
        --region us-west-2 `
        --capabilities CAPABILITY_IAM `
        --resolve-s3 `
        --no-fail-on-empty-changeset `
        --no-confirm-changeset `
        --parameter-overrides $($paramOverrides -join " ")

    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: SAM deploy failed" -ForegroundColor Red
        exit 1
    }

    # If this was the first deploy, we need a second pass to set SiteUrl
    if (-not $stackExists) {
        Write-Host "`n--- First deploy complete. Setting SiteUrl... ---" -ForegroundColor Yellow

        $newOutputs = aws cloudformation describe-stacks `
            --stack-name streammaxing `
            --region us-west-2 `
            --query "Stacks[0].Outputs" `
            --output json | ConvertFrom-Json

        $siteUrl = ($newOutputs | Where-Object { $_.OutputKey -eq "CloudFrontUrl" }).OutputValue
        Write-Host "  CloudFront URL: $siteUrl" -ForegroundColor Cyan

        # Update SiteUrl parameter and redeploy (fast - only Lambda config changes)
        $paramOverrides[-1] = "SiteUrl='$siteUrl'"

        Write-Host "  Updating Lambda with correct SiteUrl..." -ForegroundColor Yellow
        sam deploy `
            --template-file "$PSScriptRoot\template.yaml" `
            --stack-name streammaxing `
            --region us-west-2 `
            --capabilities CAPABILITY_IAM `
            --resolve-s3 `
            --no-fail-on-empty-changeset `
            --no-confirm-changeset `
            --parameter-overrides $($paramOverrides -join " ")

        if ($LASTEXITCODE -ne 0) {
            Write-Host "ERROR: SiteUrl update failed" -ForegroundColor Red
            exit 1
        }
    }

    Write-Host "Backend deployed successfully!" -ForegroundColor Green
}

# ---------------------------------------------
# GET STACK OUTPUTS
# ---------------------------------------------
Write-Host "`n--- Fetching Stack Outputs ---" -ForegroundColor Yellow

$stackOutputs = aws cloudformation describe-stacks `
    --stack-name streammaxing `
    --region us-west-2 `
    --query "Stacks[0].Outputs" `
    --output json 2>&1 | ConvertFrom-Json

if (-not $stackOutputs) {
    Write-Host "ERROR: Could not fetch stack outputs. Is the stack deployed?" -ForegroundColor Red
    exit 1
}

$cloudFrontUrl = ($stackOutputs | Where-Object { $_.OutputKey -eq "CloudFrontUrl" }).OutputValue
$cloudFrontDistId = ($stackOutputs | Where-Object { $_.OutputKey -eq "CloudFrontDistributionId" }).OutputValue
$bucketName = ($stackOutputs | Where-Object { $_.OutputKey -eq "FrontendBucketName" }).OutputValue
$apiGatewayUrl = ($stackOutputs | Where-Object { $_.OutputKey -eq "ApiGatewayUrl" }).OutputValue

Write-Host "  CloudFront URL:     $cloudFrontUrl" -ForegroundColor Cyan
Write-Host "  Distribution ID:    $cloudFrontDistId" -ForegroundColor Cyan
Write-Host "  S3 Bucket:          $bucketName" -ForegroundColor Cyan
Write-Host "  API Gateway URL:    $apiGatewayUrl" -ForegroundColor Cyan

# ---------------------------------------------
# FRONTEND DEPLOYMENT
# ---------------------------------------------
if (-not $BackendOnly) {
    Write-Host "`n--- Building Frontend ---" -ForegroundColor Yellow

    Push-Location "$PSScriptRoot\frontend"

    # Install dependencies if needed
    if (-not (Test-Path "node_modules")) {
        npm install
    }

    # Build with production env (VITE_API_URL empty = same origin via CloudFront)
    $env:VITE_API_URL = ""
    $env:VITE_DISCORD_CLIENT_ID = $envVars['DISCORD_CLIENT_ID']
    npm run build

    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Frontend build failed" -ForegroundColor Red
        Pop-Location
        exit 1
    }
    Pop-Location

    Write-Host "Frontend built: frontend/dist/" -ForegroundColor Green

    # Upload to S3
    Write-Host "`n--- Deploying Frontend to S3 ---" -ForegroundColor Yellow

    aws s3 sync "$PSScriptRoot\frontend\dist" "s3://$bucketName" `
        --delete `
        --region us-west-2

    # Set cache headers: index.html = no cache, assets = long cache
    aws s3 cp "s3://$bucketName/index.html" "s3://$bucketName/index.html" `
        --metadata-directive REPLACE `
        --cache-control "no-cache, no-store, must-revalidate" `
        --content-type "text/html" `
        --region us-west-2

    Write-Host "Frontend uploaded to S3" -ForegroundColor Green

    # Invalidate CloudFront cache
    Write-Host "`n--- Invalidating CloudFront Cache ---" -ForegroundColor Yellow

    aws cloudfront create-invalidation `
        --distribution-id $cloudFrontDistId `
        --paths "/*" `
        --region us-west-2 | Out-Null

    Write-Host "CloudFront cache invalidation started" -ForegroundColor Green
}

# ---------------------------------------------
# SUMMARY
# ---------------------------------------------
Write-Host "`n============================================" -ForegroundColor Cyan
Write-Host "  DEPLOYMENT COMPLETE!" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Your app is live at:" -ForegroundColor White
Write-Host "  $cloudFrontUrl" -ForegroundColor Green
Write-Host ""
Write-Host "IMPORTANT - Update OAuth redirect URIs:" -ForegroundColor Yellow
Write-Host "  Discord Developer Portal:" -ForegroundColor White
Write-Host "    $cloudFrontUrl/api/auth/discord/callback" -ForegroundColor Cyan
Write-Host "  Twitch Developer Console:" -ForegroundColor White
Write-Host "    $cloudFrontUrl/api/auth/twitch/callback" -ForegroundColor Cyan
Write-Host ""
Write-Host "To add a custom domain later, see .agent/SOP/deployment.md" -ForegroundColor DarkGray
Write-Host ""
