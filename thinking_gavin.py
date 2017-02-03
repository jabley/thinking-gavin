import os

from flask import Flask, request, jsonify

import requests

app = Flask(__name__)

USERNAME = os.environ.get('MG_USERNAME')
PASSWORD = os.environ.get('MG_PASSWORD')
GAVIN_ID = '16191858'

MG_URL = 'http://version1.api.memegenerator.net/Instance_Create'

def get_meme(image_id, text0, text1):
    params = {
        'username': USERNAME,
        'password': PASSWORD,
        'languageCode': 'en',
        'text0': text0 or '',
        'text1': text1 or '',
        'imageID': image_id,
        'generatorID': '6693723'
    }
    response = requests.get(MG_URL, params=params)
    if response.ok:
        r_json = response.json()
        try:
            img_url = r_json['result']['instanceImageUrl']
        except Exception as e:
            raise Exception(r_json) from e

        return img_url
def_params = {'image_id': GAVIN_ID}

@app.route("/<image_id>/", methods=['POST'])
@app.route("/", methods=['POST'])
def thinking(image_id=GAVIN_ID):
    args = request.form.get('text').split(':')
    img = None
    if len(args) > 1:
        img = get_meme(image_id, args[0], args[1])
    else:
        img = get_meme(image_id, args[0], None)

    if img:
        return jsonify(**{
            'response_type': 'in_channel',
            'attachments': [
                {
                    'text': img,
                    'image_url': img
                }
            ]
        })
    else:
        return None

if __name__ == '__main__':
    print("Using {username} for memegenerator api".format(
        username=USERNAME)
    )
    port = os.environ.get('PORT', 8999)
    app.run(host='0.0.0.0', port=port)
