var sid = "%d"

function Server() {
    //hideRequestTypeForms();
    //console.log('server');
    console.log(document.getElementById('cmd').value); //input cmd

    var cmd = document.getElementById('cmd').value;
    var oldText = document.getElementById('server-response-text').innerHTML;

    //fetch send cmd
    var myHeaders = new Headers();
    myHeaders.append("Content-Type", "text/plain");
    myHeaders.append("Session-Id", sid);

    var raw = cmd;
    
    var requestOptions = {
      method: 'POST',
      headers: myHeaders,
      body: raw,
      redirect: 'follow'
    };
    
	fetch("http://localhost:8080/consoleCMD", requestOptions)
	.then(response => response.text())
	.then(result => {
		console.log("response from server: " + result);
		var serverResponse = result + '<br>' + oldText;
		document.getElementById('server-response-text').innerHTML = serverResponse;
	})
	.catch(error => console.log('error', error));


    
};



function Database() {
    //hideRequestTypeForms();
    //var queryResponse = document.getElementById('query-response').value;
    var query = document.getElementById('sql-query').value;

    console.log("User query: " + query);
    //console.log("Response from db server: " + queryResponse);



    var myHeaders = new Headers();
    myHeaders.append("Content-Type", "text/plain");
    myHeaders.append("Session-Id", sid);

    var raw = query;

    var requestOptions = {
    method: 'POST',
    headers: myHeaders,
    body: raw,
    redirect: 'follow'
    };

	fetch("http://localhost:8080/DBquery", requestOptions)
	.then(response => response.text())
	.then(result => {
		console.log("MySQL DB Server Response: " + result);
		document.getElementById('query-response').innerHTML = result;
	})
	.catch(error => console.log('error', error));

    
};
