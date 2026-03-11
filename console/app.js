const API = "https://api.aiprotocol.uk/admin";
const ADMIN_KEY = "CHANGE_ME_ADMIN_KEY";

async function loadJSON(path){
  try {
    const res = await fetch(API + path, {
      headers: { "X-Admin-Key": ADMIN_KEY }
    });
    return await res.json();
  } catch (e) {
    return { ok:false, error:String(e) };
  }
}

function renderTokens(items){
  const el = document.getElementById("tokens");
  if (!items || !items.length) {
    el.innerHTML = "<p class='small'>No tokens yet.</p>";
    return;
  }
  el.innerHTML = items.map(x => `
    <div class="row">
      <div>
        <strong>${x.name}</strong><br>
        <span class="small">${x.token_masked}</span>
      </div>
      <div>${x.credits} credits</div>
    </div>
  `).join("");
}

function renderJobs(items){
  const el = document.getElementById("jobs");
  if (!items || !items.length) {
    el.innerHTML = "<p class='small'>No jobs yet.</p>";
    return;
  }
  el.innerHTML = items.map(x => `
    <div class="row">
      <div>
        <strong>${x.job_id}</strong><br>
        <span class="small">${x.target}</span>
      </div>
      <div>
        ${x.status}<br>
        <span class="small">${x.visa_id || "-"}</span>
      </div>
    </div>
  `).join("");
}

(async () => {
  const tokens = await loadJSON("/tokens");
  const jobs = await loadJSON("/recent");
  renderTokens(tokens.items || []);
  renderJobs(jobs.recent || []);
})();
