import os
import jwt
import datetime
import requests
import logging
from flask import Flask, request, jsonify

logging.basicConfig(
    format="%(created)f - %(levelname)s - %(message)s",
    level=logging.DEBUG
)

# Ensure required environment variables are set
SECRET_KEY = os.getenv("SECRET_KEY")

app = Flask(__name__)

def decode_jwt(token):
    """Decode and validate JWT token."""
    try:
        decoded_token = jwt.decode(token, SECRET_KEY, options={"verify_signature": False})  # algorithms=["HS256"]

        # Retrieve the issued-at timestamp
        issued_at = decoded_token.get("iat")
        if not issued_at:
            # Missing issued-at claim
            return None

        # Convert issued_at (an integer) to a datetime object
        issued_at = datetime.datetime.fromtimestamp(issued_at, datetime.timezone.utc)

        # Check if the token is older than 1 hour
        if datetime.datetime.now(datetime.timezone.utc) > issued_at + datetime.timedelta(hours=1):
            return None  # Token expired
        
        return decoded_token
    except jwt.exceptions.JWTError:
        return None

@app.route("/", methods=["POST"])
def authorization():
    """Protected endpoint that returns a secret if the user has access."""
    proceed = True
    message = "Forwarded"
    code = 200
    
    logging.info(f"Received request with headers: {request.headers}")
    logging.info(f"\n\nReceived request with data: {request.json}")

    auth_header = request.headers.get("Authorization")
    if not auth_header or not auth_header.startswith("Bearer "):
        proceed = False
        message = "Missing or invalid token"
        code = 401
    else:
        token = auth_header.split(" ")[1]
        decoded_token = decode_jwt(token)

    if code == 200 and not decoded_token:
        proceed = False
        message = "Invalid token"
        code = 401
    
    if code == 200 and decoded_token and not decoded_token.get("has_access"):
        proceed = False
        message = "Access denied"
        code = 403

    response = jsonify({"client": decoded_token.get("sub") if decoded_token else None, **request.json, "message": message, "do-verification": proceed})
    response.status_code = code
    headers = {'X-Custom-Header': 'MyValue'}
    return (response, 200, headers)

if __name__ == "__main__":
    if not SECRET_KEY:
        raise ValueError("Missing required environment variables: SECRET_KEY")

    app.run(host='0.0.0.0', port=8080)
