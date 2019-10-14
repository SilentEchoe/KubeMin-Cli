import docker
client = docker.from_env()
container = client.containers.run('gitciimages',detach=True)

