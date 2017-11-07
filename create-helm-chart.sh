if ! test -z $TRAVIS_TAG; then
  cd helm; make package; cd ..
  ssh-keyscan $TFG_GITLAB >> ~/.ssh/known_hosts 
  git clone $HELM_REPO_GIT
  cp -r \helm/pkg/* helm-repo/charts
  cd helm-repo; make vendor; make upload-with-env
  git add .; git commit -m "travis helm update"; git push origin master
fi
