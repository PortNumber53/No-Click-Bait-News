// Declarative Pipeline for No-Click Bait News
// Builds Go backend for amd64 + arm64, deploys amd64 to web1,
// and deploys the Cloudflare Worker frontend.

pipeline {
  agent any

  options {
    timestamps()
    skipDefaultCheckout(false)
  }

  environment {
    GO111MODULE = 'on'

    // Deployment targets
    TARGET_HOST     = 'web1'
    TARGET_DIR      = '/var/www/vhosts/api-ncbnews.portnumber53.com'
    SERVICE_NAME    = 'ap-ncbnews-backend'
    SSH_CREDENTIALS = 'brain-jenkins-private-key'

    // Database
    DATABASE_URL = credentials('prod-database-url-ncbnews')

    // JWT / Auth
    JWT_SECRET_KEY = credentials('prod-jwt-secret-ncbnews')

    // Stripe
    STRIPE_SECRET_KEY              = credentials('prod-stripe-secret-key-ncbnews')
    STRIPE_WEBHOOK_SECRET          = credentials('prod-stripe-webhook-secret-ncbnews')
    STRIPE_WEBHOOK_SECRET_THIN     = credentials('prod-stripe-webhook-secret-thin-ncbnews')
    STRIPE_WEBHOOK_SECRET_SNAPSHOT = credentials('prod-stripe-webhook-secret-snapshot-ncbnews')

    // CORS
    ALLOWED_ORIGINS = credentials('prod-allowed-origins-ncbnews')

    // Cloudflare
    CF_API_TOKEN          = credentials('cloudflare-api-token')
    CLOUDFLARE_ACCOUNT_ID = credentials('cloudflare-account-id')

    // Backend origin the Cloudflare Worker proxies /api/* to
    BACKEND_ORIGIN = credentials('prod-backend-url-ncbnews')
  }

  stages {
    stage('Checkout') {
      steps {
        checkout scm
        sh 'git rev-parse --short HEAD'
      }
    }

    stage('Build Matrix') {
      matrix {
        axes {
          axis {
            name 'GOARCH'
            values 'amd64', 'arm64'
          }
        }
        stages {
          stage('Build') {
            steps {
              dir('backend') {
                sh label: 'Go build', script: '''
                  set -euo pipefail
                  go version || true
                  export GOOS=linux
                  export CGO_ENABLED=0
                  echo "Building for $GOOS/$GOARCH"
                  out="api-ncbnews-backend-${GOOS}-${GOARCH}"
                  go build -ldflags="-s -w" -o "$out" .
                '''
              }
            }
          }
          stage('Archive') {
            steps {
              sh '''
                set -euo pipefail
                mkdir -p artifacts
                cp backend/api-ncbnews-backend-linux-${GOARCH} artifacts/
              '''
              stash name: "bin-${GOARCH}", includes: "artifacts/api-ncbnews-backend-linux-${GOARCH}"
            }
          }
        }
        post {
          success {
            echo "Built ${GOARCH} successfully"
          }
        }
      }
    }

    stage('Deploy (amd64 → web1)') {
      steps {
        unstash "bin-amd64"
        sshagent(credentials: [env.SSH_CREDENTIALS]) {
          sh label: 'Upload & install', script: '''
set -euo pipefail
BIN_LOCAL="artifacts/api-ncbnews-backend-linux-amd64"

# Upload binary to /tmp on target
scp "$BIN_LOCAL" grimlock@${TARGET_HOST}:/tmp/api-ncbnews-backend

# Generate systemd unit file
bash deploy/generate-api-ncbnews-backend-service.sh "${TARGET_DIR}" api-ncbnews-backend.service

# Upload unit file
scp api-ncbnews-backend.service grimlock@${TARGET_HOST}:/tmp/api-ncbnews-backend.service

# Generate .env for the service
cat > /tmp/api-ncbnews-backend.env <<ENVFILE
DATABASE_URL=${DATABASE_URL}
JWT_SECRET_KEY=${JWT_SECRET_KEY}
STRIPE_SECRET_KEY=${STRIPE_SECRET_KEY}
STRIPE_WEBHOOK_SECRET=${STRIPE_WEBHOOK_SECRET}
STRIPE_WEBHOOK_SECRET_THIN=${STRIPE_WEBHOOK_SECRET_THIN}
STRIPE_WEBHOOK_SECRET_SNAPSHOT=${STRIPE_WEBHOOK_SECRET_SNAPSHOT}
ALLOWED_ORIGINS=${ALLOWED_ORIGINS}
PORT=21011
ENVFILE
scp /tmp/api-ncbnews-backend.env grimlock@${TARGET_HOST}:/tmp/api-ncbnews-backend.env
rm -f /tmp/api-ncbnews-backend.env

# Prepare target and (re)start service
ssh grimlock@${TARGET_HOST} "
  set -euo pipefail
  sudo mkdir -p ${TARGET_DIR} ${TARGET_DIR}/logs
  sudo chown -R grimlock:grimlock ${TARGET_DIR}
  sudo mv /tmp/api-ncbnews-backend ${TARGET_DIR}/api-ncbnews-backend
  sudo mv /tmp/api-ncbnews-backend.env ${TARGET_DIR}/.env
  sudo chown grimlock:grimlock ${TARGET_DIR}/api-ncbnews-backend ${TARGET_DIR}/.env
  sudo chmod 0755 ${TARGET_DIR}/api-ncbnews-backend
  sudo chmod 0600 ${TARGET_DIR}/.env
  sudo mv /tmp/api-ncbnews-backend.service /etc/systemd/system/${SERVICE_NAME}.service
  sudo systemctl daemon-reload
  sudo systemctl enable ${SERVICE_NAME}
  sudo systemctl restart ${SERVICE_NAME}
"
          '''
        }
      }
    }

    stage('Deploy Worker (Cloudflare)') {
      steps {
        dir('frontend') {
          sh label: 'Deploy Cloudflare Worker', script: '''
            set -euo pipefail

            test -n "${CF_API_TOKEN:-}" || { echo "Missing CF_API_TOKEN"; exit 1; }
            test -n "${CLOUDFLARE_ACCOUNT_ID:-}" || { echo "Missing CLOUDFLARE_ACCOUNT_ID"; exit 1; }

            npm ci
            npm run build

            # Push secrets to Cloudflare Worker
            node -e 'const s=k=>process.env[k]||"";console.log(JSON.stringify({
              BACKEND_ORIGIN:s("BACKEND_ORIGIN")
            }))' | npx wrangler secret bulk

            # Deploy Worker
            npx wrangler deploy
          '''
        }
      }
    }
  }

  post {
    success { echo 'Pipeline completed successfully.' }
    failure { echo 'Pipeline failed.' }
    always  { echo 'Pipeline finished.' }
  }
}
