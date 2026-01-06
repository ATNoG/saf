import os
import requests
import logging
from flask import Flask, request, jsonify

logging.basicConfig(
    format="%(created)f - %(levelname)s - %(message)s",
    level=logging.DEBUG
)

# Configuration
DIRECTUS_URL = os.getenv("DIRECTUS_URL")
DIRECTUS_TOKEN = os.getenv("DIRECTUS_TOKEN")
K_SINK = os.getenv("K_SINK")
MAX_AMOUNT = float(os.getenv("MAX_AMOUNT"))  # Maximum transaction amount before requiring OTP Secret
OTP_SECRET = os.getenv("OTP_SECRET")  # OTP SECRET required for transactions above MAX_AMOUNT

HEADERS_REMOVE = ("Ce-Id", "Ce-Specversion", "Ce-Type", "Ce-Source", "Content-Type", "Host", "X-K-Sink")

app = Flask(__name__)

def get_user_id(username, req_id):
    url = f"{DIRECTUS_URL}/items/users?filter[username][_eq]={username}"
    headers = {"Authorization": f"Bearer {DIRECTUS_TOKEN}", "Ce-Id": req_id}
    response = requests.get(url, headers=headers)
    if response.status_code == 200 and (data := response.json().get("data")):
        return data[0]["id"]
    return None

@app.route("/", methods=["POST"])
def create_transaction():
    proceed = True
    message = "Forwarded"
    code = 200
    amount = 0
    
    logging.info(f"Received request with headers: {request.headers}")
    logging.info(f"\n\nReceived request with data: {request.json}")

    data = request.get_json()
    try:
        amount = float(data.get("amount"))
    except (TypeError, ValueError):
        proceed = False
        message = "Invalid amount provided"
        code = 400

    client = data.get("client")              # Sender's username
    destination_client = data.get("destination_client")  # Receiver's username
    otp = data.get("otp")                    # OTP provided, if any

    # Check transaction limit with OTP verification.
    if code == 200 and amount > MAX_AMOUNT and str(otp) != str(OTP_SECRET):
        proceed = False
        message = "OTP required or incorrect"
        code = 403

    # Get user IDs
    from_id = get_user_id(client, request.headers.get("Ce-Id"))
    to_id = get_user_id(destination_client, request.headers.get("Ce-Id"))
    if code == 200 and not from_id or not to_id:
        proceed = False
        message = "Invalid client usernames"
        code = 400

    response = jsonify({"message": message, "message": message, "from": from_id, "to": to_id, "do-transaction": proceed, **data})
    response.status_code = code
    return response


if __name__ == "__main__":
    if not DIRECTUS_URL or not DIRECTUS_TOKEN or not MAX_AMOUNT or not OTP_SECRET:
        raise ValueError("Missing required environment variables: DIRECTUS_URL, DIRECTUS_TOKEN, MAX_AMOUNT, OTP_SECRET")
    if not K_SINK:
        logging.warning("Missing K_SINK variable")
    app.run(host='0.0.0.0', port=8080)
