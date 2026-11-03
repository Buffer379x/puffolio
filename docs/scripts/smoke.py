#!/usr/bin/env python3
"""Destructive-to-test-data-only smoke test for a fresh local Puffolio container."""
import argparse, hashlib, http.cookiejar, io, json, os, secrets, urllib.request, urllib.error, urllib.parse, zipfile
from pathlib import Path

class Client:
    def __init__(self, origin):
        self.origin = origin.rstrip('/')
        self.jar = http.cookiejar.CookieJar()
        self.http = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))
        self.csrf = None
    def request(self, path, method='GET', payload=None, status=None, raw=None, content_type=None):
        headers = {'Origin': self.origin}
        if self.csrf: headers['X-CSRF-Token'] = self.csrf
        body = raw
        if payload is not None:
            body = json.dumps(payload).encode(); headers['Content-Type'] = 'application/json'
        if content_type: headers['Content-Type'] = content_type
        req = urllib.request.Request(self.origin + path, data=body, headers=headers, method=method)
        try: response = self.http.open(req, timeout=30)
        except urllib.error.HTTPError as error: response = error
        data = response.read()
        if (status is None and not 200 <= response.status < 300) or (status is not None and response.status != status):
            # Do not emit response bodies: they may contain session or invitation tokens.
            raise AssertionError(f'{method} {path.split("?")[0]}: expected {status}, got {response.status}')
        if 'application/json' in response.headers.get('Content-Type',''):
            return json.loads(data) if data else {}
        return data
    def upload(self, path, name, data, media_type):
        boundary = 'puffolio-' + secrets.token_hex(16)
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{name}"\r\nContent-Type: {media_type}\r\n\r\n'.encode() + data + f'\r\n--{boundary}--\r\n'.encode())
        return self.request(path, 'POST', raw=body, content_type='multipart/form-data; boundary='+boundary)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--url', required=True)
    parser.add_argument('--setup-code-file')
    parser.add_argument('--state-file')
    modes=parser.add_mutually_exclusive_group()
    modes.add_argument('--verify-state', help='Verify offline restore and revoked sessions')
    modes.add_argument('--verify-restart', help='Verify original volume and surviving session')
    parser.add_argument('--backup-file')
    args=parser.parse_args()
    if urllib.parse.urlsplit(args.url).hostname not in ('127.0.0.1','localhost','::1'):
        parser.error('Smoke tests may only target a loopback test instance.')
    client=Client(args.url)
    client.request('/healthz');client.request('/readyz')
    api='/api/v1'
    if args.verify_state or args.verify_restart:
        saved=json.loads(Path(args.verify_state or args.verify_restart).read_text())
        assert not client.request(api+'/setup/status')['setupRequired']
        req=urllib.request.Request(args.url.rstrip('/')+api+'/me',headers={'Cookie':saved['oldCookie']})
        try:
            with urllib.request.urlopen(req,timeout=10) as response:
                assert args.verify_restart and response.status==200, 'Restore must revoke old sessions'
        except urllib.error.HTTPError as error:
            assert args.verify_state and error.code==401, 'Restart must retain the original session'
        me=client.request(api+'/auth/login','POST',{'username':'smoke-admin','password':saved['password']})
        client.csrf=me['csrfToken']
        restored=client.request(api+'/liquids/'+saved['liquidId'])
        assert restored['name']=='Smoke Updated' and restored['favorite'] and restored['values']==saved['values']
        assert client.request(api+'/liquids')['total']==saved['liquidCount']
        assert len(client.request(api+'/fields')['items'])==saved['fieldCount']
        assert client.request(api+f'/fields/{saved["tagFieldId"]}/tags')['items']==saved['tags']
        image=client.request(api+'/media/'+saved['mediaId'])
        assert hashlib.sha256(image).hexdigest()==saved['imageSHA256']
        Client(args.url).request(api+'/media/'+saved['mediaId'],status=401)
        assert client.request(api+'/preferences')==saved['preferences']
        assert client.request(api+'/dashboard')==saved['dashboard']
        assert client.request(api+'/dashboard/query','POST',{'widgets':saved['dashboard']['widgets']})==saved['dashboardResult']
        checks=['login','values-date-tags','private-image','counts','preferences','dashboard-widgets',
                'session-persisted' if args.verify_restart else 'sessions-revoked']
        print(json.dumps({'status':'PASS','mode':'restart' if args.verify_restart else 'restore','checks':checks}))
        return
    if not args.setup_code_file: parser.error('--setup-code-file is required for a fresh smoke test')
    assert client.request(api+'/setup/status')['setupRequired'], 'Use a fresh disposable test volume.'
    password=secrets.token_urlsafe(24)
    me=client.request(api+'/setup','POST',{'code':Path(args.setup_code_file).read_text().strip(),'username':'smoke-admin','displayName':'Smoke','password':password,'starterFields':False})
    client.csrf=me['csrfToken']
    assert client.request(api+'/me')['user']['role']=='admin'
    field_specs=[('text',{'searchable':True}),('longtext',{}),('rating',{'stars':5}),('image',{}),('tags',{}),('color',{}),('date',{}),('number',{'scale':3,'unit':'ml'}),('select',{'options':[{'id':'first','label':'Erste'}]}),('boolean',{})]
    fields={}
    for type_,config in field_specs:
        fields[type_]=client.request(api+'/fields','POST',{'label':'Smoke '+type_,'type':type_,'required':False,'visibility':{'form':True,'detail':True,'list':True,'grid':True},'config':config})
    tag=client.request(api+f'/fields/{fields["tags"]["id"]}/tags','POST',{'name':'Prüfung','paletteId':'teal'})
    # A synthetic 4:3 PNG generated from stdlib, never a user's uploaded image.
    import struct,zlib
    def chunk(t,d):return struct.pack('>I',len(d))+t+d+struct.pack('>I',zlib.crc32(t+d)&0xffffffff)
    png=b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',4,3,8,2,0,0,0))+chunk(b'IDAT',zlib.compress((b'\0'+b'\x80\xc0\x40'*4)*3))+chunk(b'IEND',b'')
    upload=client.upload(api+'/media','synthetic.png',png,'image/png')
    media=client.request(api+f'/media/{upload["id"]}/crop','POST',{'x':0,'y':0,'width':1,'height':1})
    assert media['id']!=upload['id']
    values={'text':'Wörtlich %_ Äpfel','longtext':'Notiz\nZweite Zeile','rating':4,'image':media['id'],'tags':[tag['id']],'color':'#AABBCC','date':'2028-02-29','number':'0.000','select':fields['select']['config']['options'][0]['id'],'boolean':False}
    typed={fields[t]['id']:{'type':t,'value':v} for t,v in values.items()}
    liquid=client.request(api+'/liquids','POST',{'name':'Smoke Liquid','favorite':True,'values':typed})
    updated=client.request(api+f'/liquids/{liquid["id"]}','PATCH',{'name':'Smoke Updated','version':liquid['version']})
    assert updated['values']==liquid['values'], 'Patch must preserve unsubmitted values.'
    client.request(api+f'/liquids/{liquid["id"]}','PATCH',{'name':'Stale','version':liquid['version']},status=409)
    page=client.request(api+'/liquids?search=%25_&favorite=true&page=1&pageSize=24')
    assert len(page['items'])==1 and page['items'][0]['id']==liquid['id']
    other=client.request(api+'/admin/users','POST',{'username':'smoke-other','displayName':'Other','role':'user'})
    second=Client(args.url)
    second_me=second.request(api+'/auth/invite','POST',{'token':other['invitationToken'],'password':secrets.token_urlsafe(24)})
    second.csrf=second_me['csrfToken']
    second.request(api+f'/liquids/{liquid["id"]}',status=404)
    second.request(api+f'/media/{media["id"]}',status=404)
    assert not second.request(api+'/liquids')['items']
    client.request(api+f'/liquids/{liquid["id"]}','DELETE',{'version':updated['version']})
    trashed=client.request(api+'/liquids?trash=true')['items'][0]
    restored=client.request(api+f'/liquids/{liquid["id"]}/restore','POST',{'version':trashed['version']})
    assert restored['id']==liquid['id']
    preferences=client.request(api+'/preferences')
    preferences.update({'theme':'dark','timezone':'Europe/Berlin','view':'list','images':False,
                        'listFields':[fields['date']['id'],fields['number']['id']],
                        'gridFields':[fields['rating']['id']],
                        'roles':{'image':fields['image']['id'],'quantity':fields['number']['id']}})
    client.request(api+'/preferences','PUT',preferences)
    layout=client.request(api+'/dashboard')
    widgets=[{'id':'smoke-count','type':'count','title':'Persisted smoke count','size':'medium'},
             {'id':'smoke-tags','type':'tags','title':'Persisted smoke tags','size':'wide',
              'fieldId':fields['tags']['id'],'limit':3}]
    client.request(api+'/dashboard','PUT',{'version':layout['version'],'widgets':widgets})
    archive=client.request(api+'/exports','POST',{})
    assert isinstance(archive,bytes) and zipfile.is_zipfile(io.BytesIO(archive))
    preview=client.upload(api+'/imports/preview','puffolio.zip',archive,'application/zip')
    assert preview['counts']['liquids']==1
    client.request(api+'/imports','POST',{'token':preview['token'],'confirm':True})
    assert client.request(api+'/liquids')['total']==2
    preferences=client.request(api+'/preferences')
    dashboard=client.request(api+'/dashboard')
    assert preferences['theme']=='dark' and preferences['view']=='list' and not preferences['images']
    assert preferences['timezone']=='Europe/Berlin' and len(preferences['listFields'])==2
    assert [w['title'] for w in dashboard['widgets']]==['Persisted smoke count','Persisted smoke tags']
    dashboard_result=client.request(api+'/dashboard/query','POST',{'widgets':dashboard['widgets']})
    assert dashboard_result['items'][0]['count']==2
    assert dashboard_result['items'][1]['tags'][0]['name']=='Prüfung'
    assert dashboard_result['items'][1]['tags'][0]['count']==1
    backup=client.request(api+'/admin/backups','POST',{'password':password})
    assert isinstance(backup,bytes) and zipfile.is_zipfile(io.BytesIO(backup))
    if args.backup_file:
        target=Path(args.backup_file);target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(backup);target.chmod(0o600)
    if args.state_file:
        image=client.request(api+'/media/'+media['id'])
        saved={'password':password,'liquidId':liquid['id'],'mediaId':media['id'],'values':restored['values'],'oldCookie':'; '.join(c.name+'='+c.value for c in client.jar),'imageSHA256':hashlib.sha256(image).hexdigest(),
               'tagFieldId':fields['tags']['id'],'tags':client.request(api+f'/fields/{fields["tags"]["id"]}/tags')['items'],
               'liquidCount':2,'fieldCount':len(client.request(api+'/fields')['items']),
               'preferences':preferences,'dashboard':dashboard,'dashboardResult':dashboard_result}
        state_path=Path(args.state_file);state_path.write_text(json.dumps(saved));state_path.chmod(0o600)
    print(json.dumps({'status':'PASS','fields':10,'checks':['health','setup','typed-values','crop-revision','patch-preservation','version-conflict','literal-search','two-user-isolation','trash-restore','collection-roundtrip','nondefault-preferences-dashboard','instance-backup']}))
if __name__=='__main__':main()
