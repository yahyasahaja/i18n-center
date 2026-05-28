// PLEASE NOTE THAT IF YOU'RE MAKING A CHANGE INTO THE PIPELINE SCRIPT HERE, PLEASE UPDATE ACCORDINGLY INTO
// http://10.104.0.4:8080/job/<ENV>/job/i18n-center-kubernetes/configure
//
// This pipeline builds TWO images from a single repo:
//   - asia-docker.pkg.dev/lapakgaming-production/lapakgaming/i18n-center/backend
//   - asia-docker.pkg.dev/lapakgaming-production/lapakgaming/i18n-center/frontend
//
// Both are tagged with the short commit hash and the same tag is written into
// products/lapakgaming/environments/<env>/i18n-center/values.yaml via .global.image.tag.
// The backend image contains BOTH the server binary and the migrate CLI; the
// migrate Job (ArgoCD PreSync) reuses the backend image with `command:` overridden.

pipeline {
  agent any
  tools {
      go 'go-1.23'
  }
  environment {
    REPO_LOCATION = "asia"
    CREDENTIAL_KEY = "lapakgaming-artifact-registry-sa-key"
    COMMIT_HASH = ""
    IMAGE_YAML_FILE = "products/lapakgaming/environments/dev/i18n-center/values.yaml"
    KUBERNETES_REPO_URL = "https://github.com/lapakgaming/kubernetes-gcp.git"
    KUBERNETES_CLONE_DIR = "."
    KUBERNETES_CLONE_REPO_NAME = "kubernetes-gcp"
    YQ = "/usr/local/bin/yq"
  }
  stages {
    stage('Clone Repo') {
      steps {
        checkout([$class: 'GitSCM', branches: [[name: "$params.Version"]], userRemoteConfigs: [[credentialsId: 'github-token', url: 'https://github.com/lapakgaming/i18n-center.git']]])
      }
    }

    stage('Unit test stage') {
      steps {
        echo 'running backend unit tests'
        sh 'make test'
      }
    }

    stage('Build') {
      steps {
        sh 'make build'
        // login into docker artifact registry
        withCredentials([file(credentialsId: "${CREDENTIAL_KEY}", variable: 'GCR_CRED')]) {
          sh 'cat "${GCR_CRED}" | docker login -u _json_key --password-stdin https://${REPO_LOCATION}-docker.pkg.dev'
        }
        sh 'make push'
        sh "docker logout https://${REPO_LOCATION}-docker.pkg.dev"
      }
    }

    stage('Deploy') {
      steps {
        script {
          COMMIT_HASH = sh(returnStdout: true, script: 'git show -q --format=%h').trim()
        }

        echo "update kubernetes-gcp image tag..."
        sh "git clone -b ${params.kubernetesBranch} --single-branch --depth 1 ${KUBERNETES_REPO_URL} ${KUBERNETES_CLONE_DIR}/${KUBERNETES_CLONE_REPO_NAME}"
        dir("${KUBERNETES_CLONE_DIR}/${KUBERNETES_CLONE_REPO_NAME}") {
          sh "${YQ} eval '.global.image.tag = \"${COMMIT_HASH}\"' -i ${IMAGE_YAML_FILE}"
          sh "git add ${IMAGE_YAML_FILE}"
          script {
            def diffOutput = sh(returnStdout: true, script: 'git diff --staged').trim()

            if (diffOutput) {
                echo "image tag difference detected: ${diffOutput}"
                echo "push changes..."
                sh 'git commit -m "[deployment-pipeline] Update i18n-center image tag in values.yaml"'
                sh "git push origin ${params.kubernetesBranch}"
            } else {
                echo 'no changes...'
            }
          }
        }

        echo "cleaning up jenkins workspace..."
        sh 'make cleanup'
      }
    }
  }

  post {
    always {
      deleteDir() /* clean up our workspace */
    }
  }
}
