if ! test -z $TRAVIS_TAG; then
  cd helm; make package; cd ..
  ssh-keyscan $TFG_GITLAB >> ~/.ssh/known_hosts 
  git clone $HELM_REPO_GIT > /dev/null 2>&1
  cp -r \helm/pkg/* helm-repo/charts
  cd helm-repo; make vendor; make upload-with-env > /dev/null 2>&1
  git add .; git commit -m "travis helm update"; git push origin master > /dev/null 2>&1
fi
