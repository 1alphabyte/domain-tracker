export default async function getTLSCert(d) {
	const proc = Bun.spawn(["./fetch_certificate", d]);
	// check if error
	if (await proc.exited != 0) {
		throw new Error("external depend error: The error lies outside of JS\n" + proc.stderr);
	}
	let text = await new Response(proc.stdout).text();
	text = text.split("!").map((t) => t.trim());
	// check all required parts are present
	if (text.length < 4) {
		throw new Error("Something went wrong");
	}
	return text;
}