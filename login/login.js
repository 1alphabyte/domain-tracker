if (localStorage.getItem("auth")) {
	location.assign("/dash/");
} else {
	document.querySelector("form").addEventListener("submit", (e) => {
		e.preventDefault();
		const input = document.getElementById("username");
		const password = document.getElementById("password");
		fetch("/api/login", {
			method: "POST",
			body: JSON.stringify({ username: input.value, password: password.value })
		}).then(res => {
			if (res.ok) {
				window.location.assign("/dash/");
				localStorage.setItem("auth", true);
			} else if (res.status === 403) {
				alert("Invalid username or password");
			} else {
				alert("An error occurred");
			}
		});
	});
	console.info("Made with ❤️ by @1alphabyte https://github.com/1alphabyte");
}