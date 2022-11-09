var $ = function(id) { return document.getElementById(id); }; //shorthand for document.getElementById command

var orderArray = [
    %s
]

buildTable(orderArray)

var j = 0;

var sid = "%d"

function buildTable(data){
    var table = document.getElementById('ordersTable')

    console.log(orderArray);
    console.log("^ orderArray from GetDBInitInfo, contains each requests Id from DB (currently hardcoded)");

    for(i = 0; i < data.length; i++) {
        var row = `<tr>
                <td>${data[i].Piepr}</td>
                <td> 
                    <a data-bs-toggle="modal" data-bs-target="#modal-assembly" href="#" id=${data[i].Id}>${data[i].Darb1}</a>
                </td>
                <td>
                <a data-bs-toggle="modal" data-bs-target="#modal-assembly" href="#" id=emailId${data[i].Id}>${data[i].Darb2}</a>
                </td>
                <td>
                    <a data-bs-toggle="modal" data-bs-target="#modal-assembly" href="#" id=updateId${data[i].Id}>${data[i].Darb3}</a>
                </td>
            </tr>`
        j++;
        table.innerHTML += row    
    }
}

var hideRequestTypeForms = function() {
    $('complectation-form').style.display = 'none';
    $('purchace-form').style.display = 'none';
    $('update-status-form').style.display = 'none';
    $('comment-form').style.display = 'none';

  }
  hideRequestTypeForms();

var elementId;
document.addEventListener('click', (e) => {
     elementId = e.target.id;
     console.log(elementId);

    // Retrieve id from clicked element - See Details - GetByIndex
    if (!isNaN(elementId) && elementId.length > 0) {
        
        // Send GetByIndex?id={val} request
        var myHeaders = new Headers();
        myHeaders.append("Content-Type", "application/json");
        myHeaders.append("Session-Id", sid);
        var raw = JSON.stringify({});

        var requestOptions = {
            method: 'GET',
            headers: myHeaders,
            //body: raw,
            redirect: 'follow'
        };

        var requestId = elementId;
        console.log("Clicked on id: " + requestId);
        
        var responseJson;

        fetch("http://localhost:8080/GetByIndex?id=" + requestId, requestOptions)
        .then(response => response.text())
        .then(result =>  { 
                console.log("GetByIndex " + requestId + " POST response: " + result);
                
                var json = JSON.parse(result);

                if (json.reqType=="Complectation") {
                    hideRequestTypeForms();
                    $('complectation-form').style.display = 'block';
                    $("name").innerHTML = json.name;
                    $("case").innerHTML = json.case;
                    $("motherboard").innerHTML = json.motherboard;
                    $("cpu").innerHTML = json.cpu;
                    $("videocard").innerHTML = json.videocard;
                    $("ram").innerHTML = json.ram;
                    $("memory").innerHTML = json.memory;
                    $("tel").innerHTML = json.tel; 
                    $("deliv").innerHTML = json.deliv;
                    $("status").innerHTML = json.status;

                    $("email").innerHTML = json.email;
                    $("notes").innerHTML = json.notes;
                    $("date").innerHTML = json.date;
                }
                else {
                    hideRequestTypeForms();
                    $('purchace-form').style.display = 'block';
                    $("repair-name").innerHTML = json.name;
                    $("component-type").innerHTML = json.componentType;
                    $("model").innerHTML = json.model;
                    $("repair-tel").innerHTML = json.tel;
                    $("repair-deliv").innerHTML = json.deliv;
                    $("repair-status").innerHTML = json.status;

                    $("repair-email").innerHTML = json.email;
                    $("repair-notes").innerHTML = json.problem;
                    $("repair-date").innerHTML = json.date;
                }
            })
        .catch(error => console.log('error', error));
    }

    if (elementId.includes("updateId")) {
        hideRequestTypeForms();
        $('update-status-form').style.display = 'block'; 
        $('update-status-form-id').value = elementId;
    }

    if (elementId.includes("emailId")) {
        hideRequestTypeForms();
        $('comment-form').style.display = 'block'; 
        
        
        var myHeaders = new Headers();
        myHeaders.append("Content-Type", "text/plain");
        myHeaders.append("Session-Id", sid);

        var requestOptions = {
            method: 'GET',
            headers: myHeaders,
            redirect: 'follow'
          };

        //fetch("http://localhost:8080/email", requestOptions)
        //.then(result => function(r) {
        //    $('comment-form-mail').value = r;
        //})
         $('comment-form-mail').value = "example@riekstins.com";

    }

    });

    

    window.addEventListener('beforeunload', function (e) {
        var myHeaders = new Headers();
        //myHeaders.append("Content-Type", "application/json");
        myHeaders.append("Session-Id", sid);

        var requestOptions = {
            method: 'POST',
            headers: myHeaders,
            body: raw,
            redirect: 'follow'
          };

        fetch("http://localhost:8080/exit", requestOptions)
 

});


function sendStatus() {
    var statusMsg = document.getElementsByName('statusType').item(0).value;
    console.log("statusmsg: " + statusMsg);

    var commentMsg = $('update-comment').value;
    console.log(commentMsg);
    
    var id = $('update-status-form-id').value.substring(8, undefined);
    console.log(id);

    //console.log("Send Status clicked: " + elementId);
    //console.log(elementId.substring(8, undefined));

     var myHeaders = new Headers();
     myHeaders.append("Content-Type", "application/json");
     myHeaders.append("Session-Id", sid);
     
     var raw = "{\"id\":" + id + ", \"status\": \""+ statusMsg +"\", \"comment\": \""+ commentMsg +"\"}";

     var requestOptions = {
       method: 'POST',
       headers: myHeaders,
       body: raw,
       redirect: 'follow'
     };
     
     fetch("http://localhost:8080/setStatus", requestOptions)
       .then(response => response.text())
       .then(result => console.log(result))
       .then(result => console.log(elementId))
       .catch(error => console.log('error', error));
} 

function sendEmail() {
    var email = $('comment-form-mail').value;
    var content = $('commentbox-msg').value;

    var myHeaders = new Headers();
    myHeaders.append("Content-Type", "application/json");
    myHeaders.append("Session-Id", sid);
    
    var raw = "{\"email\":\"" + email + "\", \"msg\":\""+ content +"\"}";

    var requestOptions = {
      method: 'POST',
      headers: myHeaders,
      body: raw,
      redirect: 'follow'
    };
    
    fetch("http://localhost:8080/sendMsg", requestOptions)
      .then(response => response.text())
      .then(result => console.log(result))
      .then(result => console.log(elementId))
      .catch(error => console.log('error', error));
}

function Exit() {
    location.reload();
}