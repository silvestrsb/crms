function Server() {
    //hideRequestTypeForms();
    console.log('server');
    var oldText = document.getElementById('server-response-text').innerHTML;

    //fetch send cmd

    var serverResponse = 'response <br>' + oldText;
    document.getElementById('server-response-text').innerHTML = serverResponse;
};

function Database() {
    //hideRequestTypeForms();
    console.log('database');
};