let data = null;   // 载入的 summary.json
let who = 'skill'; // 'skill' | 'hero'
let what = 'total';// 'total' | 'ratio'
let sortKey = 'total';
let sortAsc = false;

const $ = s => document.querySelector(s);
const $$ = s => document.querySelectorAll(s);

$('#file').addEventListener('change', async (e)=>{
    const f = e.target.files[0]; if (!f) return;
    data = JSON.parse(await f.text());
    hydrateKPIs();
    renderAll();
});

$('#whoSeg').addEventListener('click', e=>{
    if (e.target.tagName !== 'BUTTON') return;
    $$('#whoSeg button').forEach(b=>b.classList.remove('active'));
    e.target.classList.add('active');
    who = e.target.dataset.who;
    renderAll();
});
$('#whatSeg').addEventListener('click', e=>{
    if (e.target.tagName !== 'BUTTON') return;
    $$('#whatSeg button').forEach(b=>b.classList.remove('active'));
    e.target.classList.add('active');
    what = e.target.dataset.what;
    renderAll();
});

$('#tbl thead').addEventListener('click', e=>{
    if (!e.target.classList.contains('sortable')) return;
    const key = e.target.dataset.key;
    if (sortKey === key) sortAsc = !sortAsc; else { sortKey = key; sortAsc = (key==='name'); }
    renderTable();
    renderBars(); // 保持条形图排序一致
});

function hydrateKPIs() {
    const fmt = (x, d=2)=> (typeof x==='number' ? x.toFixed(d) : '-');
    $('#k_runs').textContent  = data?.runs ?? '-';
    $('#k_win').textContent   = data?.win_rate!=null ? (data.win_rate*100).toFixed(1)+'%' : '-';
    $('#k_time').textContent  = fmt(data?.avg_time, 2);
    $('#k_dps').textContent   = fmt(data?.avg_dps, 1);
    $('#k_total').textContent = fmt(data?.total_damage ?? sumValues(data?.by_skill), 0);
}

function pickDataset(){
    if (!data) return [];
    const src = who==='skill' ? (data.by_skill || {}) : (data.by_hero || {});
    const arr = Object.entries(src).map(([name, obj])=>{
        const total = typeof obj==='number' ? obj : (obj.total ?? 0);
        const ratio = (typeof obj==='object' && obj.ratio!=null) ? obj.ratio : (data.total_damage>0 ? total/data.total_damage : 0);
        return { name, total, ratio };
    });
    // 排序
    arr.sort((a,b)=>{
        const A = a[sortKey], B = b[sortKey];
        if (A===B) return a.name.localeCompare(b.name);
        return (A<B) ? (sortAsc? -1:1) : (sortAsc? 1:-1);
    });
    return arr;
}

function renderAll(){
    if (!data) return;
    $('#chartTitle').textContent = `按${who==='skill'?'技能':'角色'} · ${what==='total'?'总量':'占比'}`;
    $('#pieTitle').textContent   = `按${who==='skill'?'技能':'角色'} · 占比`;
    $('#tabTitle').textContent   = `数据表 · 按${who==='skill'?'技能':'角色'}`;
    const ds = pickDataset();
    renderBars(ds);
    renderPie(ds);
    renderTable(ds);
    $('#footnote').textContent = `总伤害：${fmt0(data.total_damage ?? sumValues(data.by_skill))} · 项目数：${ds.length}`;
}

function renderBars(ds = pickDataset()){
    const canvas = $('#bar'); const ctx = canvas.getContext('2d');
    // 处理 DPR
    const DPR = window.devicePixelRatio || 1;
    const CW = canvas.clientWidth, CH = canvas.clientHeight;
    canvas.width = CW * DPR; canvas.height = CH * DPR; ctx.scale(DPR, DPR);

    ctx.clearRect(0,0,CW,CH);
    const pad = {t:10, r:16, b:24, l: 140};
    const W = CW - pad.l - pad.r, H = CH - pad.t - pad.b;

    // 取前 N 项
    const topN = Math.min(12, ds.length);
    const rows = ds.slice(0, topN);

    const values = rows.map(r => (what==='total' ? r.total : r.ratio));
    const maxV = Math.max(1e-6, ...values);
    const barH = H / topN * 0.68;
    const gapH = H / topN * 0.32;

    ctx.font = '12px system-ui';
    ctx.textBaseline = 'middle';

    rows.forEach((r, i)=>{
        const y = pad.t + i*(barH+gapH) + barH/2;
        // 名称
        ctx.fillStyle = getCSS('--muted');
        ctx.textAlign='right';
        ctx.fillText(r.name, pad.l - 8, y);
        // 条
        const v = (what==='total'? r.total : r.ratio);
        const w = (v / maxV) * W;
        ctx.fillStyle = barColor(i);
        roundRect(ctx, pad.l, y - barH/2, Math.max(2,w), barH, 6, true, false);
        // 值
        ctx.fillStyle = getCSS('--fg'); ctx.textAlign='left';
        const txt = (what==='total') ? fmt0(v) : (v*100).toFixed(1)+'%';
        ctx.fillText(txt, pad.l + w + 6, y);
    });
}

function renderPie(ds = pickDataset()){
    const canvas = $('#pie'); const ctx = canvas.getContext('2d');
    const DPR = window.devicePixelRatio || 1;
    const CW = canvas.clientWidth, CH = canvas.clientHeight;
    canvas.width = CW * DPR; canvas.height = CH * DPR; ctx.scale(DPR, DPR);

    ctx.clearRect(0,0,CW,CH);
    const cx = CW/2, cy = CH/2, R = Math.min(CW,CH)*0.34;

    const total = ds.reduce((s,r)=> s + r.total, 0);
    if (total <= 0) return;

    let ang = -Math.PI/2;
    ds.slice(0, 16).forEach((r,i)=>{
        const p = r.total / total;
        const a2 = ang + Math.PI*2*p;
        ctx.beginPath();
        ctx.moveTo(cx,cy);
        ctx.arc(cx,cy,R, ang, a2);
        ctx.closePath();
        ctx.fillStyle = barColor(i);
        ctx.fill();

        // label
        const mid = (ang + a2)/2;
        const lx = cx + Math.cos(mid)*(R+16);
        const ly = cy + Math.sin(mid)*(R+16);
        ctx.fillStyle = getCSS('--fg');
        ctx.font = '12px system-ui';
        ctx.textAlign = (Math.cos(mid)>0)? 'left':'right';
        ctx.fillText(`${r.name} ${(p*100).toFixed(1)}%`, lx, ly);
        ang = a2;
    });
}

function renderTable(ds = pickDataset()){
    const tbody = $('#tbody'); tbody.innerHTML = '';
    ds.forEach(r=>{
        const tr = document.createElement('tr');
        tr.innerHTML = `
      <td>${escapeHTML(r.name)}</td>
      <td>${fmt0(r.total)}</td>
      <td>${(r.ratio*100).toFixed(2)}%</td>
    `;
        tbody.appendChild(tr);
    });
}

function sumValues(obj){
    if (!obj) return 0;
    let s = 0;
    for (const k in obj){
        const v = obj[k];
        if (typeof v === 'number') s += v;
        else if (v && typeof v.total === 'number') s += v.total;
    }
    return s;
}

// utils
function fmt0(n){ return (n||0).toLocaleString(); }
function getCSS(name){ return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }
function barColor(i){
    const palette = ['#60a5fa','#34d399','#fbbf24','#f472b6','#a78bfa','#f87171','#2dd4bf','#fb7185','#93c5fd','#facc15','#4ade80','#fca5a5','#c084fc','#22d3ee','#f59e0b','#10b981'];
    return palette[i % palette.length];
}
function roundRect(ctx, x,y,w,h,r, fill=true, stroke=false){
    if (w<2*r) r = w/2; if (h<2*r) r = h/2;
    ctx.beginPath();
    ctx.moveTo(x+r,y); ctx.arcTo(x+w,y,x+w,y+h,r); ctx.arcTo(x+w,y+h,x,y+h,r);
    ctx.arcTo(x,y+h,x,y,r); ctx.arcTo(x,y,x+w,y,r); ctx.closePath();
    if (fill) ctx.fill(); if (stroke) ctx.stroke();
}
function escapeHTML(s){ return s.replace(/[&<>"']/g, m=>({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;' }[m])); }
