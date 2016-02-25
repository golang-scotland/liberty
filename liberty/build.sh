#! /bin/bash

verbose='false'
appenv=''
push=false

while getopts 'e:pv' flag; do
  case "${flag}" in
    e) appenv="${OPTARG}" ;;
    p) push=true ;;
    v) verbose='true' ;;
    *) error "Unexpected option ${flag}" ;;
  esac
done

[[ -z "$appenv" ]] && { echo "You must give supply the 'appenv' '-e' argument" ; exit 1; }
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $DIR
go build .
docker build --rm -t registry.golang.scot/liberty:${appenv} -f $DIR/${appenv}.Dockerfile $DIR

if [ "$push" = true ] ; then
	docker push registry.golang.scot/liberty:${appenv}
fi
