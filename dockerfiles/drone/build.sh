#! /bin/bash

verbose='false'
appenv=''
push=false
deploy=false

while getopts 'e:pdvg:' flag; do
  case "${flag}" in
    e) appenv="${OPTARG}" ;;
    p) push=true ;;
    d) deploy=true ;;
    v) verbose='true' ;;
	g) gover="${OPTARG}" ;;
    *) error "Unexpected option ${flag}" ;;
  esac
done

[[ -z "$appenv" ]] && { echo "You must give supply the 'appenv' '-e' argument" ; exit 1; }

DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $DIR

docker build --rm -t golang.scot/drone $DIR

