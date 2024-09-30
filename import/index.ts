import fs from 'fs';
import csv from 'csv-parser';

const results: CsvRow[] = [];
const csvFilePath = './domains.csv';
let clients: string[] = [];
let apiBaseUrl = 'http://54.218.183.201:81';
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
					'Cookie': 'auth=2544a154-faa1-454c-bef1-45947014d262'
				},
				body: JSON.stringify({ name: c })
			});
		});
		fetch(`${apiBaseUrl}/api/clientList`, {
			method: 'GET',
			headers: {
				'Content-Type': 'application/json',
				'Cookie': 'auth=2544a154-faa1-454c-bef1-45947014d262'
			}
		}).then((res) => res.json()).then((data) => {
			(async () => {
				for (const r of results) {
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
							'Cookie': 'auth=2544a154-faa1-454c-bef1-45947014d262'
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

