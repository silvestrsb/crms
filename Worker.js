var $ = function(id) { return document.getElementById(id); }; //shorthand for document.getElementById command

var orderArray = [
    %s
]

buildTable(orderArray)

var j = 0;

function buildTable(data){
    var table = document.getElementById('ordersTable')

    console.log(orderArray);
    //TODO: hardcoded GO template values (current harcoded Id = 5 for all orders) to real db values
    console.log("^ orderArray from GetDBInitInfo, contains each requests Id from DB (currently hardcoded)");

    for(i = 0; i < data.length; i++) {
        var row = `<tr>
                <td>${data[i].Piepr}</td>
                <td> 
                    <a data-bs-toggle="modal" data-bs-target="#modal-assembly" href="#" id=${data[i].Id}>${data[i].Darb1}</a>
                </td>
                <td>${data[i].Darb2}</td>
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
  }
  hideRequestTypeForms();


document.addEventListener('click', (e) => {
     let elementId = e.target.id;
     //console.log(elementId);

    // Retrieve id from clicked element - See Details - GetByIndex
    if (!isNaN(elementId) && elementId.length > 0) {
        
        // Send GetByIndex?id={val} request
        var myHeaders = new Headers();
        myHeaders.append("Content-Type", "application/json");
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
       console.log("Update Status clicked: " + elementId);
        /* TODO: updateStatus method 
        hideRequestTypeForms();
        $('update-form').style.display = 'block'; //TODO


        var myHeaders = new Headers();
        myHeaders.append("Content-Type", "application/json");

        var raw = JSON.stringify({});

        var requestOptions = {
        method: 'PUT',
        headers: myHeaders,
        body: raw,
        redirect: 'follow'
        };

        fetch("http://localhost:8080/UpdateById?=" + id, requestOptions)
        .then(response => response.text())
        .then(result => console.log(result))
        .catch(error => console.log('error', error));
        */
    }
 
});
