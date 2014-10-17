DEMO_NETWORK ?= 127.0.0
net := $(DEMO_NETWORK)
tag := consul-demo-node

clean:
	docker rm -f server1 || :
	docker rm -f server2 || :
	docker rm -f server3 || :
	docker rm -f agent1 || :
	docker rm -f agent2 || :
	docker rm -f agent3 || :

erase:	clean
	sudo docker rmi $(tag)

cluster:
	docker run -d --volume=$(CURDIR)/dc1:/etc/consul --name=server1 --hostname=server1 -p=$(net).20:53:8600/udp -p=$(net).20:80:8500 $(tag) -server -bootstrap
	docker run -d --volume=$(CURDIR)/dc1:/etc/consul --link=server1:peer --name=server2 --hostname=server2 -p=$(net).21:80:8500 $(tag) -server -join=peer
	docker run -d --volume=$(CURDIR)/dc1:/etc/consul --link=server2:peer --name=server3 --hostname=server3 -p=$(net).22:80:8500 $(tag) -server -join=peer
	docker run -d --volume=$(CURDIR)/dc1:/etc/consul --link=server3:peer --name=agent1 --hostname=agent1 -p=$(net).30:80:8500 $(tag) -join=peer
	docker run -d --volume=$(CURDIR)/dc1:/etc/consul --link=agent1:peer --name=agent2 --hostname=agent2 -p=$(net).31:80:8500 $(tag) -join=peer
	docker run -d --volume=$(CURDIR)/dc1:/etc/consul --link=agent2:peer --name=agent3 --hostname=agent3 -p=$(net).32:80:8500 $(tag) -join=peer

image:	
	docker build -t=$(tag) .

info:
	echo "Buildinfo for $(tag)"
	docker images $(tag)

