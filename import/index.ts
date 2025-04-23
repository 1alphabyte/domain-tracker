import fs from 'fs';
import csv from 'csv-parser';

const results: CsvRow[] = [];
const csvFilePath = './domains.csv';
let clients: string[] = [];
let apiBaseUrl = 'http://localhost:8080';
let authCookie = "auth="
fs.createReadStream(csvFilePath)
	.pipe(csv())
	.on('data', (data: CsvRow) => results.push(data))
	.on('end', () => {
		results.forEach((r: { Client: string, domain: string, Notes: String }) => {
			if (clients.indexOf(r.Client) === -1) {
				clients.push(r.Client);
			}
		})
		clients.forEach((c: string) => {
			fetch(`${apiBaseUrl}/api/clientAdd`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'Cookie': authCookie
				},
				body: JSON.stringify({ name: c })
			});
		});
		let currDomains;
		fetch(`${apiBaseUrl}/api/clientList`, {
			method: 'GET',
			headers: {
				'Content-Type': 'application/json',
				'Cookie': authCookie
			}
		}).then((res) => res.json()).then((data) => {
			(async () => {
				currDomains = await fetch(`${apiBaseUrl}/api/get`, {
					headers: {
						'Content-Type': 'application/json',
						'Cookie': authCookie
					}}).then(r => r.json());
					console.log(results.length);
				for (const r of results) {
					if (currDomains.filter((d: { domain: string }) => d.domain === r.domain).length != 0) {
						continue;
					}
					let cId: number = data.filter((d: { name: string }) => d.name === r.Client);
					let body: String = JSON.stringify({
						domain: r["domain"],
						clientId: cId[0].id,
						notes: r.Notes || ""
					});
					console.log(body);
					await fetch(`${apiBaseUrl}/api/add`, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/json',
							'Cookie': authCookie
						},
						body: body
					}).then((res) => {
						console.log(res);
					});
					await Bun.sleep(1000 * 25);
				}
			})();
		}).catch((err) => {
			console.error(err);
		})
	})

