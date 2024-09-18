import { Database } from "bun:sqlite";
import Bun from "bun";
import rdapClient from "rdap-client"
import whoiser from "whoiser"

const db = new Database("./db.sqlite");

function checkAuth(req) {
	let token = req.headers.get("Cookie");
	if (!token) 
		return null;
	
	token = token.split("=");
	if (token[0] != "auth")
		return null;

	let session = db.query("SELECT * FROM sessions WHERE token = ?1;").get(token[1]);
	if (!session || session.expires < Date.now())
		return null;
	return session;
}

const server = Bun.serve({
	async fetch(req) {
		let pathArr = req.url.replace(/https?:\/\//, "").split("/");
		let path = pathArr[2];
		switch (path) {
			case "login": {
				if (req.method != "POST") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "POST" } });
				}
				if (!req.body) {
					return new Response("Bad request", { status: 400 });
				}
				let body = await req.json();
				if (!body.username || !body.password) {
					return new Response("Bad request", { status: 400 });
				}
				let user = db.query("SELECT * FROM users WHERE username = ?1")
					.get(body.username);
				let validLogin;
				if (!user) {
					validLogin = false;
				} else {
					validLogin = await Bun.password.verify(body.password, user.password);
				}
				if (!validLogin) {
					return new Response("Invalid username or password", { status: 403 })
				}

				// Create new session
				let q = db.query("INSERT INTO sessions (token, userId, expires) VALUES (?1, ?2, ?3) RETURNING token").get(crypto.randomUUID(), user.id, Date.now() + (48 * 60 * 60 * 1000));		
				return new Response("Session created", { status: 201, headers: { "Set-Cookie": `auth=${q.token}; Max-Age=172800; Path=/; httpOnly; SameSite=Lax` } });
			}
			case "get": {
				if (req.method != "GET") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "GET" } });
				}
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				let q = db.query("SELECT * FROM domains").all();
				return new Response(JSON.stringify(q), { headers: { "Content-Type": "application/json" } });
			}
			case "edit": {
				if (req.method != "POST")
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "GET" } });
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				if (!req.body)
					return new Response("Missing body", { status: 400 });
				let body = await req.json();
				if (!body.id || !body.clientId || !body.expiration || !body.nameservers)
					return new Response(null, { status: 400 });
				try {
					db.query("UPDATE domains SET expiration = ?1, nameservers = ?2, clientId = ?3, notes = ?4 WHERE id = ?5")
						.run(new Date(body.expiration).getTime(), body.nameservers, body.clientId, body.notes || "", body.id);
				} catch (error) {
					console.error(error);
					return new Response("Error updating domain", { status: 500 });
				}
				return new Response(null, { status: 200 });
			}
			case "add": {
				if (req.method != "POST") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "POST" } });
				}
				let session = checkAuth(req);
				if (!session) 
					return new Response("Unauthorized", { status: 401 });
				if (!req.body) 
					return new Response("Missing body", { status: 400 });
				
				let body = await req.json();
				if (!body.domain || !body.clientId)
					return new Response("Missing domain or client ID", { status: 400 });
				let exp, ns, reg, raw;
				try {
					// All gTLDs are supported by RDAP
					// RDAP is the preferred method for getting domain data as it is machine readable
					let rdapData = await rdapClient.rdapClient(body.domain);
					exp = new Date(rdapData.events.filter((event) => event.eventAction === "expiration")[0].eventDate).getTime();
					ns = rdapData.nameservers.map((ns) => ns.ldhName).toString();
					reg = rdapData.entities[0].vcardArray[1][1][3]
					raw = JSON.stringify(rdapData);
				} catch (e) {
					console.error(e);
					console.info("Trying whoiser", body.domain);
					try {
						let whoisData = await whoiser.domain(body.domain, { follow: 1, ignorePrivacy: false });
						whoisData = Object.values(whoisData)[0];
						exp = new Date(whoisData["Expiry Date"]).getTime();
						ns = whoisData["Name Server"].toString();
						reg = whoisData.Registrar;
						raw = JSON.stringify(whoisData);
					} catch (e) {
						console.error(e);
						return new Response("Error getting domain data", { status: 500 });
					}
				}
				let q
				try {
					q = db.query("INSERT INTO domains (domain, expiration, nameservers, registrar, clientId, rawWhoisData, notes) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7);")
						.run(
							body.domain,
							exp,
							ns,
							reg,
							body.clientId,
							raw,
							body.notes || ""
						).lastInsertRowid;
				} catch (error) {
					console.error(error);
					return new Response("Error adding domain", { status: 500 });
				}
				return new Response(q, { status: 201 });
			}
			case "clientList": {
				if (req.method != "GET") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "GET" } });
				}
				let session = checkAuth(req);
				if (!session) 
					return new Response("Unauthorized", { status: 401 });
				let q = db.query("SELECT * FROM clients").all();
				return new Response(JSON.stringify(q), { headers: { "Content-Type": "application/json" } });
			}
			case "clientAdd": {
				if (req.method != "POST") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "POST" } });
				}
				let session = checkAuth(req);
				if (!session) 
					return new Response("Unauthorized", { status: 401 });
				if (!req.body) 
					return new Response("Missing body", { status: 400 });
				
				let body = await req.json();
				if (!body.name)
					return new Response("Missing name", { status: 400 });
				let q = db.query("INSERT INTO clients (name) VALUES (?1);")
					.run(body.name).lastInsertRowid;
				return new Response(q, { status: 201 });
			}
			case "delete": {
				if (req.method != "DELETE") 
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "DELETE" } });
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				let id = pathArr[3];
				if (!id)
					return new Response("Missing ID", { status: 400 });
				try {
					db.query("DELETE FROM domains WHERE id = ?1").run(id);
				} catch (error) {
					console.error(error);
					return new Response("Error deleting domain", { status: 500 });
				}
				return new Response(null, { status: 200 });
			}
			case "deleteClient": {
				if (req.method != "DELETE") 
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "DELETE" } });
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				let id = pathArr[3];
				if (!id)
					return new Response("Missing ID", { status: 400 });
				// check if there are domains are linked with client
				let q = db.query('SELECT COUNT(*) AS domainCount FROM domains WHERE clientId = ?').get(id);
				if (q.domainCount > 0) {
					return new Response("Client has linked domains and cannot be deleted", { status: 409 });
				}
			  
				try {
					db.query("DELETE FROM clients WHERE id = ?1").run(id);
				} catch (error) {
					console.error(error);
					return new Response("Error deleting client", { status: 500 });
				}
				return new Response(null, { status: 200 });
			}
			default: {
				pathArr.shift()
				return new Response(`Page: /${pathArr.join("/")}, Not found`, { status: 404 });
			}
		}
	},
 });

console.log("Listening on:", server.url.host)