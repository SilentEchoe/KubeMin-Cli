import asyncio
from aiohttp import web


async def index(request):
    await asyncio.sleep(1)
    return  web.Response(body=b'<h1>Index</h1>')


async def hello(request):
    await asyncio.sleep(1)
    text = '<h1>hello, %s!</h1>' % request.match_info['name']
    return web.Response(body=text.encode('utf-8'))


async def init(loop):
    app = web.Application(loop=loop)
    app.router.add_route('GET', '/', index)

