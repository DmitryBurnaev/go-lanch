export $(cat .env | grep -v ^# | xargs)
docker build -t go-lunch .
docker run -ti --volume $(pwd)/bin:/app/bin go-lunch
ssh ${SSH_HOST} 'supervisorctl stop go-lunch'
scp bin/go-lunch ${SSH_HOST}:/var/www/go-lunch
ssh ${SSH_HOST} 'chmod u+x /var/www/go-lunch'
ssh ${SSH_HOST} 'supervisorctl start go-lunch'
