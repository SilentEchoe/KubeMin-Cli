import docker

client = docker.from_env()
client.containers.run("microsoft/aspnetcore", detach=True)
