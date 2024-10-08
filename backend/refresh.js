import { Database } from "bun:sqlite";
import nodemailer from "nodemailer";
import rdapClient from "rdap-client"
import whoiser from "whoiser"

const db = new Database(process.env.DB_PATH || "./db.sqlite");

let transporter = nodemailer.createTransport({
	host: "send.smtp.com",
	port: 465,
	secure: true,
	auth: {
		user: process.env.SMTP_USER,
		pass: process.env.SMTP_PASSWORD,
	},
});

let c;

db.query("SELECT * FROM sessions").all().forEach((session) => {
	if (session.expires < Date.now()) {
		db.query("DELETE FROM sessions WHERE token = ?1").run(session.token);
		c++;
	}
});

if (!isNaN(c)) {
	console.info(`Deleted ${c} expired session${c === 1 ? "" : "s"}`);
}

const domains = db.query("SELECT * FROM domains").all();
for (const domain of domains) {
	let domainDate = new Date(domain.expiration).getTime();
	let date = new Date().getTime() + (25 * 24 * 60 * 60 * 1000);
	if (domainDate < date) {
		console.info("Refreshing", domain.domain);
		let exp, ns, reg, raw;
		try {
			// All gTLDs are supported by RDAP
			// RDAP is the preferred method for getting domain data as it is machine readable
			let rdapData = await rdapClient.rdapClient(domain.domain);
			if (rdapData.events)
				exp = new Date(rdapData.events.filter((event) => event.eventAction === "expiration")[0].eventDate).getTime();
			if (rdapData.nameservers)
				ns = rdapData.nameservers.map((ns) => ns.ldhName).toString();
			if (rdapData.entities)
				reg = rdapData.entities.filter((r) => r.roles[0] === "registrar")[0].vcardArray[1][1][3]
			raw = JSON.stringify(rdapData);
		} catch (e) {
			console.error(e);
			console.info("Trying whoiser", domain.domain);
			try {
				let whoisData = await whoiser.domain(domain.domain, { follow: 1, ignorePrivacy: false });
				whoisData = Object.values(whoisData)[0];
				exp = new Date(whoisData["Expiry Date"]).getTime();
				ns = whoisData["Name Server"].toString();
				reg = whoisData.Registrar;
				raw = JSON.stringify(whoisData);
			} catch (e) {
				console.error(e);
				console.info("Error getting domain", domain.domain);
				continue;
			}
		}
		try {
			db.query("UPDATE domains SET expiration = ?1, nameservers = ?2, registrar = ?3, rawWhoisData = ?4 WHERE id = ?5")
				.run(exp, ns, reg, raw, domain.id);
			console.info("Updated", domain.domain);
		} catch (e) {
			console.error(e);
			console.info("Error updating domain", domain.domain);
		}
		await Bun.sleep(1000 * 25);
	}
}

let expiringSoon = [];

db.query("SELECT * FROM domains").all().forEach((domain) => {
	let domainDate = new Date(domain.expiration).getTime();
	let date = new Date().getTime() + (25 * 24 * 60 * 60 * 1000);
	if (domainDate < date) {
		expiringSoon.push(domain);
	}
});

if (expiringSoon.length != 0) {

	const currentDate = new Date();

	transporter.sendMail({
		from: "domaintrk@cbt.io",
		to: process.env.EMAIL_FOR_EXPIRING_DOMAINS,
		subject: "Domains expiring soon",
		text: "    ,---,                        ____                                              \n  .'  .' `\\                    ,'  , `.             ,--,                           \n,---.'     \\    ,---.       ,-+-,.' _ |           ,--.'|         ,---,             \n|   |  .`\\  |  '   ,'\\   ,-+-. ;   , ||           |  |,      ,-+-. /  | .--.--.    \n:   : |  '  | /   /   | ,--.'|'   |  || ,--.--.   `--'_     ,--.'|'   |/  /    '   \n|   ' '  ;  :.   ; ,. :|   |  ,', |  |,/       \\  ,' ,'|   |   |  ,\"' |  :  /`./   \n'   | ;  .  |'   | |: :|   | /  | |--'.--.  .-. | '  | |   |   | /  | |  :  ;_     \n|   | :  |  ''   | .; :|   : |  | ,    \\__\\/: . . |  | :   |   | |  | |\\  \\    `.  \n'   : | /  ; |   :    ||   : |  |/     ,\" .--.; | '  : |__ |   | |  |/  `----.   \\ \n|   | '` ,/   \\   \\  / |   | |`-'     /  /  ,.  | |  | '.'||   | |--'  /  /`--'  / \n;   :  .'      `----'  |   ;/        ;  :   .'   \\;  :    ;|   |/     '--'.     /  \n|   ,.'                '---'         |  ,     .-./|  ,   / '---'        `--'---'   \n'---'                                 `--`---'     ---`-'                          \nexpiring soon\n"
			+ expiringSoon.map((d) => {
				const expirationDate = new Date(d.expiration);

				return `${d.domain} is expiring on ${expirationDate.toLocaleDateString()} (in ${Math.ceil((expirationDate - currentDate) / (1000 * 60 * 60 * 24))} days)`;
			}).join("\n")
	}, (err, info) => {
		if (err) {
			console.error(err);
		} else {
			console.info(info);
		}
	});
};
