import subprocess


#P = subprocess.call('dotnet -v', shell=True)

l = subprocess.call('dotnet -v', shell=True,stdin=subprocess.PIPE, stdout=0ww  )
print(l)

