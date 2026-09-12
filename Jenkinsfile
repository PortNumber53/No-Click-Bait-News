// Declarative Pipeline for NoClickBait News
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
    TARGET_DIR      = '/var/www/vhosts/api-ncbnews.truvis.co'
    SERVICE_NAME    = 'api-ncbnews-backend'
    RUNTIME_ENV_FILE = '/etc/ncbnews/backend.env'
    SSH_CREDENTIALS = 'brain-jenkins-private-key'

    // CI-only credentials used by Jenkins to publish the frontend Worker.
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

    stage('Quality Gates') {
      parallel {
        stage('Backend tests') {
          steps {
            dir('backend') {
              sh '''
                set -euo pipefail
                go vet ./...
                go test -race ./...
              '''
            }
          }
        }
        stage('Frontend checks') {
          steps {
            dir('frontend') {
              sh '''
                set -euo pipefail
                npm ci
                npm run lint
                npm audit
                npx wrangler types --check
                npm run build
              '''
            }
          }
        }
        stage('Mobile checks') {
          when {
            expression { sh(script: 'command -v flutter >/dev/null 2>&1', returnStatus: true) == 0 }
          }
          steps {
            dir('mobile') {
              sh '''
                set -euo pipefail
                flutter pub get
                flutter analyze --no-pub
                flutter test --no-pub
              '''
            }
          }
        }
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

    stage('Deploy & Migrate (amd64 → web1)') {
      steps {
        unstash "bin-amd64"
        sshagent(credentials: [env.SSH_CREDENTIALS]) {
          sh label: 'Upload & install', script: '''
set -euo pipefail
BIN_LOCAL="artifacts/api-ncbnews-backend-linux-amd64"

# Upload binary to /tmp on target
scp "$BIN_LOCAL" "grimlock@$TARGET_HOST:/tmp/api-ncbnews-backend"

# Generate systemd unit file. Runtime configuration is host-managed and is
# deliberately never copied through Jenkins.
bash deploy/generate-ncbnews-backend-service.sh "$TARGET_DIR" api-ncbnews-backend.service "$RUNTIME_ENV_FILE"

# Generate the hourly news crawler cron definition.
bash deploy/generate-news-crawler-cron.sh "$TARGET_DIR" ncbnews-news-crawler.cron

# Upload service, crawler, and cron files.
scp api-ncbnews-backend.service "grimlock@$TARGET_HOST:/tmp/api-ncbnews-backend.service"
scp deploy/run-news-crawler.sh "grimlock@$TARGET_HOST:/tmp/run-news-crawler.sh"
scp ncbnews-news-crawler.cron "grimlock@$TARGET_HOST:/tmp/ncbnews-news-crawler.cron"

# Migrate using the target host's protected environment, then replace the
# binary and roll back if health verification fails.
ssh "grimlock@$TARGET_HOST" "
  set -euo pipefail
  test -r $RUNTIME_ENV_FILE
  chmod 0755 /tmp/api-ncbnews-backend
  set -a
  source $RUNTIME_ENV_FILE
  set +a
  /tmp/api-ncbnews-backend migrate
  sudo systemctl stop $SERVICE_NAME 2>/dev/null || true
  sudo mkdir -p $TARGET_DIR $TARGET_DIR/logs
  sudo chown -R grimlock:grimlock $TARGET_DIR
  if [ -f $TARGET_DIR/api-ncbnews-backend ]; then
    cp $TARGET_DIR/api-ncbnews-backend $TARGET_DIR/api-ncbnews-backend.previous
  fi
  sudo mv /tmp/api-ncbnews-backend $TARGET_DIR/api-ncbnews-backend
  sudo chown grimlock:grimlock $TARGET_DIR/api-ncbnews-backend
  sudo chmod 0755 $TARGET_DIR/api-ncbnews-backend
  sudo mv /tmp/api-ncbnews-backend.service /etc/systemd/system/$SERVICE_NAME.service
  sudo mv /tmp/run-news-crawler.sh $TARGET_DIR/run-news-crawler.sh
  sudo chown grimlock:grimlock $TARGET_DIR/run-news-crawler.sh
  sudo chmod 0755 $TARGET_DIR/run-news-crawler.sh
  sudo mv /tmp/ncbnews-news-crawler.cron /etc/cron.d/ncbnews-news-crawler
  sudo chown root:root /etc/cron.d/ncbnews-news-crawler
  sudo chmod 0644 /etc/cron.d/ncbnews-news-crawler
  sudo systemctl daemon-reload
  sudo systemctl enable $SERVICE_NAME
  sudo systemctl enable --now cronie
  sudo systemctl start $SERVICE_NAME
  healthy=false
  for attempt in 1 2 3 4 5 6; do
    if curl --fail --silent http://127.0.0.1:21011/health >/dev/null; then healthy=true; break; fi
    sleep 2
  done
  if [ \"\$healthy\" != true ]; then
    sudo systemctl stop $SERVICE_NAME || true
    if [ -f $TARGET_DIR/api-ncbnews-backend.previous ]; then
      mv $TARGET_DIR/api-ncbnews-backend.previous $TARGET_DIR/api-ncbnews-backend
      sudo systemctl start $SERVICE_NAME
    fi
    exit 1
  fi
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

            # Deploy Worker with static assets
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
