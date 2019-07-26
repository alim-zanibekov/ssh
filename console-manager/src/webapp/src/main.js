const div = document.createElement('div');
div.innerHTML = `
    <div>Agents list:</div> <br>
    <div class="agents" id="agents">
        
    </div>
`;

document.body.appendChild(div);


fetch('/agents-list')
    .then(res => res.json())
    .then(res => {
        const agentsBlock = document.getElementById('agents');
        res.forEach(item => {
            const a = document.createElement('a');
            a.innerText = item;
            a.href = '#';
            a.style.display = "block";
            a.onclick = () => {
                document.body.removeChild(div);
                window.agentId = item;
                import(/* webpackChunkName: "terminal" */'./terminal');
                return false;
            };
            agentsBlock.appendChild(a);
        });
    });