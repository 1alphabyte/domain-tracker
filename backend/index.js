import { Database } from "bun:sqlite";
import Bun from "bun";
import rdapClient from "rdap-client";
import whoiser from "whoiser";
import getTLSCert from "./fetch_crt.js";
const db = new Database(process.env.DB_PATH || "./db.sqlite");


function checkAuth(req) { // Checks if the user is authenticated using a Cookie
	let cookie = req.headers.get("Cookie");
	if (!cookie) 
		return null;

	cookie = cookie.split(";")
	let token = cookie.filter((e) => e.trimStart().startsWith("auth="))

	if (token.length <= 0)
		return null;

	token = token[0]
	token = token.split("=");


	let session = db.query("SELECT * FROM sessions WHERE token = ?1;").get(token[1]);
	if (!session || session.expires < Date.now())
		return null;
	return session;
}


const server = Bun.serve({ // make a new server using Bun serve
	async fetch(req) {
		// Get the request path
		let pathArr = req.url.replace(/https?:\/\//, "").split("/");
		// get the last part of the request path
		let path = pathArr[2];
		// switch statement to handle different routes
		switch (path) {
			// Handle login route	
			case "login": {
				// make sure the method is POST
				if (req.method !== "POST") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "POST" } });
				}
				// make sure the request has a body
				if (!req.body)
					return new Response("Bad request", { status: 400 });
				// get the request body from the client
				let body = await req.json();
				// make sure the body has a username and password
				if (!body.username || !body.password) {
					return new Response("Bad request", { status: 400 });
				}
				// get the user from the database
				let user = db.query("SELECT * FROM users WHERE username = ?1")
					.get(body.username);
				let validLogin;
				// make sure the user exists
				if (!user) {
					validLogin = false;
				} else {
					// verify the password
					validLogin = await Bun.password.verify(body.password, user.password);
				}
				// if the login is invalid, return a 403
				if (!validLogin) {
					return new Response("Invalid username or password", { status: 403 })
				}

				// Create new session
				let q = db.query("INSERT INTO sessions (token, userId, expires) VALUES (?1, ?2, ?3) RETURNING token").get(crypto.randomUUID(), user.id, Date.now() + (48 * 60 * 60 * 1000));		
				// send back the cookie as a header
				return new Response("Session created", { status: 201, headers: { "Set-Cookie": `auth=${q.token}; Max-Age=172800; Path=/; httpOnly; SameSite=Lax; Secure` } });
			}
			case "get": { // send all domains and associated data to the client
				if (req.method !== "GET") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "GET" } });
				}
				// make sure the user is authenticated
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				// get all the domains from the database
				let q = db.query("SELECT * FROM domains").all();
				// send the domains back to the client
				return new Response(JSON.stringify(q), { headers: { "Content-Type": "application/json" } });
			}
			case "edit": { // gets the request body and updates the domain in the database
				if (req.method !== "POST")
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
			case "add": { // add a new domain to the database and do a whois lookup to get additional info
				if (req.method !== "POST") {
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
				// make variables to store domain data from WhoIS
				let exp, ns, reg, raw;
				try {
					// All gTLDs are supported by RDAP
					// RDAP is the preferred method for getting domain data as it is machine readable
					let rdapData = await rdapClient.rdapClient(body.domain);
					// parse data into correct format (UNIX)
					exp = new Date(rdapData.events.filter((event) => event.eventAction === "expiration")[0].eventDate).getTime();
					// parse data using map and other techniques
					ns = rdapData.nameservers.map((ns) => ns.ldhName).toString();
					reg = rdapData.entities.filter((r) => r.roles[0] === "registrar")[0].vcardArray[1][1][3];
					raw = JSON.stringify(rdapData);
				} catch (e) {
					// handle error by falling back to WhoIS
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
				let q;
				// fetch DNS info
				let dnsObj = JSON.stringify({
					"a": await fetch(`https://dns.google/resolve?name=${body.domain}&type=a`).then(res => res.json()).then(data => 
						(data.Status === 0 && data.Answer) ? data.Answer[0].data : null
					),
					"aaaa": await fetch(`https://dns.google/resolve?name=${body.domain}&type=aaaa`).then(res => res.json()).then(data =>
						(data.Status === 0 && data.Answer) ? data.Answer[0].data : null
					),
					"mx": await fetch(`https://dns.google/resolve?name=${body.domain}&type=mx`).then(res => res.json()).then(data =>
						(data.Status === 0 && data.Answer) ? data.Answer.map(item => item.data) : null	
					)
				});
				try {
					q = db.query("INSERT INTO domains (domain, expiration, nameservers, registrar, dns, clientId, rawWhoisData, notes) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8);")
						.run(
							body.domain,
							exp,
							ns,
							reg,
							dnsObj,
							body.clientId,
							raw,
							body.notes || ""
						).lastInsertRowid;
				} catch (error) {
					if (error.code === "SQLITE_CONSTRAINT_UNIQUE")
						return new Response("Domain already exists", { status: 409 });
					console.error(error);
					return new Response("Error adding domain", { status: 500 });
				}
				return new Response(q, { status: 201 });
			}
			case "clientList": {
				if (req.method !== "GET") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "GET" } });
				}
				let session = checkAuth(req);
				if (!session) 
					return new Response("Unauthorized", { status: 401 });
				// get all the clients from the database
				let q = db.query("SELECT * FROM clients").all();
				// send the clients back to the client
				return new Response(JSON.stringify(q), { headers: { "Content-Type": "application/json" } });
			}
			case "clientAdd": {
				if (req.method !== "POST") {
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
				// send the new client ID back to the client
				return new Response(q, { status: 201 });
			}
			case "delete": {
				if (req.method !== "DELETE")
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
				if (req.method !== "DELETE")
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "DELETE" } });
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				let id = pathArr[3];
				if (!id)
					return new Response("Missing ID", { status: 400 });
				// check if there are domains are linked with client
				let q = db.query('SELECT COUNT(*) AS domainCount FROM domains WHERE clientId = ?').get(id);
				// also check in TLS tracker
				let q2 = db.query('SELECT COUNT(*) AS tlsCount FROM crts WHERE clientId = ?').get(id);
				if (q.domainCount > 0 && q2.tlsCount > 0) {
					return new Response("Client has linked resources and cannot be deleted", { status: 409 });
				}
			  
				try {
					db.query("DELETE FROM clients WHERE id = ?1").run(id);
				} catch (error) {
					console.error(error);
					return new Response("Error deleting client", { status: 500 });
				}
				return new Response(null, { status: 200 });
			}
			case "tlsAddDomain": {
				if (req.method !== "POST")
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "DELETE" } });
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				let body = await req.json();
				if (!body.domain || !body.clientId)
					return new Response("Missing domain or client ID", { status: 400 });
				let text = await getTLSCert(body.domain)
				let q;
				try {
					q = db.query("INSERT INTO crts (domain, commonName, expiration, authority, clientId, rawData, notes) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7);")
					.run(
						body.domain,
						text[2],
						new Date(text[1]).getTime(),
						text[0],
						body.clientId,
						text[3],
						body.notes || ""
					).lastInsertRowid;
				} catch (error) {
					if (error.code === "SQLITE_CONSTRAINT_UNIQUE")
						return new Response(text[2], { status: 409 });
					console.error(error);
					return new Response("Error adding domain", { status: 500 });
				}
				return new Response(q, { status: 201 });
			}
			case "tlsList": {
				if (req.method !== "GET") {
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "GET" } });
				}
				let session = checkAuth(req);
				if (!session) 
					return new Response("Unauthorized", { status: 401 });
				// get all the clients from the database
				let q = db.query("SELECT * FROM crts").all();
				// send the clients back to the client
				return new Response(JSON.stringify(q), { headers: { "Content-Type": "application/json" } });
			}
			case "tlsDelete": {
				if (req.method !== "DELETE")
					return new Response("Method not allowed", { status: 405, headers: { "Allow": "DELETE" } });
				if (!checkAuth(req)) 
					return new Response("Unauthorized", { status: 401 });
				let id = pathArr[3];
				if (!id)
					return new Response("Missing ID", { status: 400 });
				try {
					db.query("DELETE FROM crts WHERE id = ?1").run(id);
				} catch (error) {
					console.error(error);
					return new Response("Error deleting domain", { status: 500 });
				}
				return new Response(null, { status: 200 });
			}
			default: {
				// remove host
				pathArr.shift()
				// send back 404 error
				return new Response(`Page: /${pathArr.join("/")}, Not found`, { status: 404 });
			}
		}
	},
 });

console.log("Listening on:", server.url.host)