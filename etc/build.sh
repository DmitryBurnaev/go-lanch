docker build -t go-lunch .
docker run -ti --volume ./bin:/app/bin go-lunch
ssh -c supervisorctl stop go-lunch
scp ./bin/go-lunch devpython:/home/admin/www/go-lunch
ssh -c supervisorctl start go-lunch
