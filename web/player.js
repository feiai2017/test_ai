// ===== 状态 =====
let events = [];
let t = 0, rate = 1, lastIdx = 0, playing = false;
let bossHP = 120000, maxHP = 120000;
let guard = 100, maxGuard = 100;
const actors = new Map(); // id -> {x,y,hp,color,isBoss}
const moves = []; const fx = [];
let activeId = null; // 当前前台（Switch 事件或首次 Spawn 设定）

// 单次播放内统计
const dmgByHero = new Map();     // heroId -> { total, skill, react }
const bossOutToHero = new Map(); // heroId -> boss对其伤害
const metaInfo = {
    heroNames: new Map(),
    heroMaxHp: new Map(),
    bossId: null,
    bossName: null,
    intentNames: new Map(),
    skillNames: new Map()
};
const STATUS_LABELS = {
    burning: '燃烧',
    shock: '感电',
    wet: '潮湿',
    slow: '减速',
    frostbite: '冻伤',
    weaken: '虚弱'
};
const ELEMENT_LABELS = {
    fire: '火',
    water: '水',
    ice: '冰',
    storm: '雷',
    electro: '雷',
    hydro: '水',
    pyro: '火',
    geo: '岩',
    dendro: '草',
    physical: '物理'
};
let preferLogLines = false;

// ===== DOM & Canvas =====
const $ = s=>document.querySelector(s);
const ctx = $('#stage').getContext('2d');
const logBox = $('#log');

// ===== 控件 =====
$('#speed').addEventListener('input', e=>{
    rate = parseFloat(e.target.value); $('#spval').textContent = rate.toFixed(2)+'x';
});
$('#file').addEventListener('change', async e=>{
    const f = e.target.files[0]; if (!f) return;
    const obj = JSON.parse(await f.text());
    loadMeta(obj.meta || obj.Meta || null);
    events = obj.events || obj.Events || [];
    resetWorld();
    append(`日志加载：${events.length} 条`);
});
$('#play').addEventListener('click', ()=>{ if (!events.length) return; playing = true; tick(); });
$('#stop').addEventListener('click', ()=> playing = false);

// ===== 世界重置 =====
function resetWorld(){
    t = 0; lastIdx = 0; playing = false;
    bossHP = maxHP; guard = maxGuard;
    actors.clear(); moves.length = 0; fx.length = 0; logBox.innerHTML = '';
    activeId = null;
    preferLogLines = false;
    draw(); updateHUD();
}

// ===== 主循环（高速更稳） =====
function tick(){
    if (!playing) return;
    const need = (1/60) * rate;
    t += need; $('#time').textContent = `t=${t.toFixed(2)}s`;

    while (lastIdx < events.length && normT(events[lastIdx]) <= t){
        handle(normE(events[lastIdx++]));
    }
    // 固定步长推进 FX
    let remain = need, physDT = 1/60;
    while (remain > 1e-6) { stepFX(Math.min(remain, physDT)); remain -= physDT; }

    draw();
    requestAnimationFrame(tick);
}
function normT(ev){ return ev.t ?? ev.T ?? 0; }
function normE(ev){ return { t: normT(ev), type: ev.type ?? ev.Type, payload: ev.payload ?? ev.Payload }; }

// ===== 事件处理 =====
function handle(ev){
    switch(ev.type){
        case 'Spawn': {
            const payload = ev.payload || {};
            const id = payload.id;
            if (!id) break;
            const isBoss = !!payload.boss;
            const x = toPxX(payload.x), y = toPxY(payload.y);
            const maxHpVal = payload.max_hp ?? payload.hp ?? (isBoss ? maxHP : (metaInfo.heroMaxHp.get(id) ?? 5000));
            const hpVal = payload.hp ?? maxHpVal;
            actors.set(id, { id, x, y, hp: hpVal, maxHp: maxHpVal, color: isBoss ? '#b00' : pickColor(id), isBoss });
            if (isBoss){
                maxHP = maxHpVal;
                bossHP = hpVal;
                const guardVal = payload.guard ?? guard;
                const guardMaxVal = payload.guard_max ?? maxGuard;
                guard = guardVal;
                maxGuard = guardMaxVal;
                metaInfo.bossId = id;
                if (payload.name) metaInfo.bossName = payload.name;
                if (!metaInfo.bossName) metaInfo.bossName = id;
                updateHUD();
            } else {
                if (!metaInfo.heroNames.has(id) && payload.name) metaInfo.heroNames.set(id, payload.name);
                if (maxHpVal) metaInfo.heroMaxHp.set(id, maxHpVal);
                if (!activeId) activeId = id; // 首个英雄默认前台
            }
            if (!preferLogLines){
                const hpText = `${fmt0(hpVal)}/${fmt0(maxHpVal)}`;
                const guardText = isBoss ? `，护甲 ${fmt0(guard)}/${fmt0(maxGuard)}` : '';
                logDetail(ev, `${actorLabel(id)} 登场，HP ${hpText}${guardText}`);
            }
            break;
        }
        case 'Move': {
            const a = actors.get(ev.payload.id); if (!a) break;
            const [fx,fy] = ev.payload.from, [tx,ty] = ev.payload.to;
            a.x = toPxX(tx); a.y = toPxY(ty);
            moves.push({from:{x:toPxX(fx),y:toPxY(fy)}, to:{x:a.x,y:a.y}, life:0.25});
            break;
        }
        case 'Switch': {
            const to = ev.payload.to; activeId = to;
            const target = actors.get(to);
            const nameText = displayName(to);
            if (target) spawnFXText(target.x ?? 60, (target.y ?? 60)-60, `>> ${nameText} <<`, 0.8);
            if (!preferLogLines){
                logDetail(ev, `前台切换至 ${actorLabel(to)}`);
            }
            break;
        }
        case 'Cast': {
            const c = actors.get(ev.payload.caster); if (!c) break;
            const skill = skillLabel(ev.payload.skill);
            spawnFXText(c.x, c.y-22, skill, 0.7);
            if (!preferLogLines){
                logDetail(ev, `${actorLabel(ev.payload.caster)} 施放了 ${skill}`);
            }
            break;
        }
        case 'ApplyStatus': {
            const target = actors.get(ev.payload.target); if (!target) break;
            spawnFXBadge(target.x, target.y-36, badgeFor(ev.payload.status), 1.2);
            if (!preferLogLines){
                const statusName = statusLabel(ev.payload.status);
                const dur = ev.payload.dur;
                const durText = typeof dur === 'number' ? `，持续 ${dur.toFixed(1)}s` : '';
                logDetail(ev, `${actorLabel(ev.payload.target)} 获得状态 ${statusName}${durText}`);
            }
            break;
        }
        case 'Hit': {
            const payload = ev.payload || {};
            const target = actors.get(payload.target);
            if (target){
                const dmg = payload.dmg || 0;
                const hpVal = payload.hp ?? target.hp;
                target.hp = hpVal;
                if (target.isBoss) {
                    bossHP = target.hp;
                    // 英雄技能归因：caster 是英雄
                    const hid = payload.caster;
                    if (hid && actors.has(hid) && !actors.get(hid).isBoss){
                        const rec = dmgByHero.get(hid) || {total:0, skill:0, react:0};
                        rec.total += dmg; rec.skill += dmg;
                        dmgByHero.set(hid, rec);
                    }
                } else {
                    // Boss 输出：caster 可能是 boss001
                    bossOutToHero.set(target.id, (bossOutToHero.get(target.id)||0) + dmg);
                }
                spawnHitRing(target.x, target.y);
                spawnFXText(target.x+10, target.y-10, `-${fmt0(dmg)}`, 0.6);
                updateHUD(); renderMeters();
                if (!preferLogLines){
                    const casterLabel = actorLabel(payload.caster);
                    const targetLabel = actorLabel(payload.target);
                    const elemText = elemLabel(payload.elem);
                    const elemDesc = elemText ? `${elemText}元素伤害` : '伤害';
                    logDetail(ev, `${casterLabel} 对 ${targetLabel} 造成 ${fmt0(dmg)} 点${elemDesc}（剩余 HP ${fmt0(hpVal)}）`);
                }
            } else if (!preferLogLines){
                const casterLabel = actorLabel(payload.caster);
                const targetLabel = actorLabel(payload.target);
                logDetail(ev, `${casterLabel} 命中 ${targetLabel} 造成 ${fmt0(payload.dmg || 0)} 点伤害`);
            }
            break;
        }
        case 'Reaction': {
            const reactionId = ev.payload?.id;
            const b = findBoss(); if (b) spawnFXText(b.x, b.y-48, `✦${reactionId ?? ''}`, 1.0);
            if (!preferLogLines){
                logDetail(ev, `触发元素反应 ${reactionId || '未知反应'}`);
            } else {
                append(`[${fmtTime(ev.t)}] ⚡ Reaction: ${reactionId}`);
            }
            break;
        }
        case 'ReactionDamage': {
            const payload = ev.payload || {};
            const target = actors.get(payload.target);
            if (target){
                target.hp = payload.hp ?? target.hp;
                if (target.isBoss) bossHP = target.hp;
                spawnHitRing(target.x, target.y);
                spawnFXText(target.x, target.y-24, `-${fmt0(payload.amount || 0)}`, 0.8);
                updateHUD();
            }
            const src = payload.source;
            if (src){
                const rec = dmgByHero.get(src) || {total:0, skill:0, react:0};
                const amount = payload.amount || 0;
                rec.total += amount; rec.react += amount;
                dmgByHero.set(src, rec);
            }
            renderMeters();
            if (!preferLogLines){
                logDetail(ev, `${payload.reaction || '反应伤害'} 对 ${actorLabel(payload.target)} 造成 ${fmt0(payload.amount || 0)} 点伤害`);
            } else {
                append(`[${fmtTime(ev.t)}] ⚡ ReactionDamage ${payload.reaction} -${payload.amount} by ${src}`);
            }
            break;
        }
        case 'GuardChanged': {
            guard = ev.payload.guard ?? guard; updateHUD();
            if (!preferLogLines){
                logDetail(ev, `护甲变动至 ${fmt0(guard)}`);
            } else {
                append(`[${fmtTime(ev.t)}] Guard=${guard}`);
            }
            break;
        }
        case 'PhaseEnter': {
            const phase = ev.payload?.phase;
            const b = findBoss(); if (b) spawnFXText(b.x, b.y-70, `=== Phase ${phase} ===`, 1.2);
            if (!preferLogLines){
                logDetail(ev, `进入阶段 ${phase}`);
            } else {
                append(`[${fmtTime(ev.t)}] === Phase ${phase} ===`);
            }
            break;
        }
        case 'Announce': {
            const text = ev.payload?.text ?? '';
            const b = findBoss(); if (b) spawnFXText(b.x, b.y-90, `📢 ${text}`, 1.2);
            if (!preferLogLines){
                logDetail(ev, `公告：${text}`);
            } else {
                append(`[${fmtTime(ev.t)}] 📢 ${text}`);
            }
            break;
        }
        case 'LogLine': {
            preferLogLines = true;
            const text = ev.payload?.text ?? '';
            append(`[${fmtTime(ev.t)}] ${text}`);
            break;
        }
        default:
            if (!preferLogLines){
                logDetail(ev, `事件 ${ev.type}`);
            } else {
                append(`[${fmtTime(ev.t)}] ${ev.type}`);
            }
    }
}

// ===== 渲染 =====
function draw(){
    const cvs = $('#stage'); const w = cvs.width, h = cvs.height;
    ctx.clearRect(0,0,w,h);

    // 背景网格
    ctx.strokeStyle = '#eef2f7'; ctx.lineWidth = 1;
    for (let x=0; x<w; x+=50){ ctx.beginPath(); ctx.moveTo(x,0); ctx.lineTo(x,h); ctx.stroke(); }
    for (let y=0; y<h; y+=50){ ctx.beginPath(); ctx.moveTo(0,y); ctx.lineTo(w,y); ctx.stroke(); }

    // 最近移动轨迹
    for (const m of moves){
        ctx.globalAlpha = Math.max(0, m.life / 0.25);
        ctx.strokeStyle = '#7aa6ff'; ctx.lineWidth = 2;
        ctx.beginPath(); ctx.moveTo(m.from.x, m.from.y); ctx.lineTo(m.to.x, m.to.y); ctx.stroke();
    }
    ctx.globalAlpha = 1;

    // 实体
    const party = [...actors.values()].filter(a=>!a.isBoss);
    const boss = findBoss();
    for (const a of actors.values()){
        // 影子
        ctx.fillStyle = 'rgba(0,0,0,0.08)';
        ctx.beginPath(); ctx.ellipse(a.x, a.y+8, 18, 8, 0, 0, Math.PI*2); ctx.fill();
        // 身体
        ctx.fillStyle = a.color;
        ctx.beginPath(); ctx.arc(a.x, a.y, a.isBoss? 22 : 16, 0, Math.PI*2); ctx.fill();
        ctx.strokeStyle = a.isBoss ? '#550' : '#222'; ctx.lineWidth = 2;
        ctx.beginPath(); ctx.arc(a.x, a.y, a.isBoss? 22 : 16, 0, Math.PI*2); ctx.stroke();
        // 名称
        ctx.fillStyle = '#111'; ctx.font='12px system-ui'; ctx.textAlign='center';
        ctx.fillText(a.id, a.x, a.y-24);
        // HP
        const cur = a.isBoss ? bossHP : a.hp;
        const max = a.isBoss ? maxHP  : 5000;
        ctx.fillStyle = '#e5e7eb'; ctx.fillRect(a.x-20, a.y+22, 40, 6);
        ctx.fillStyle = '#4ade80'; ctx.fillRect(a.x-20, a.y+22, 40*Math.max(0,cur/max), 6);
    }

    // 三角阵型轮廓（把三名英雄连线）
    if (party.length >= 3) {
        const [a,b,c] = party;
        ctx.strokeStyle = 'rgba(99,102,241,0.35)'; ctx.lineWidth = 2;
        ctx.beginPath(); ctx.moveTo(a.x,a.y); ctx.lineTo(b.x,b.y); ctx.lineTo(c.x,c.y); ctx.closePath(); ctx.stroke();
    }
    // 前台高亮环
    if (activeId && actors.has(activeId)) {
        const act = actors.get(activeId);
        ctx.strokeStyle = '#10b981'; ctx.lineWidth = 3;
        ctx.beginPath(); ctx.arc(act.x, act.y, act.isBoss? 26 : 20, 0, Math.PI*2); ctx.stroke();
    }

    // 前景特效
    for (const p of fx){ p.draw(ctx); }
}

// ===== HUD / 伤害仪表 =====
function updateHUD(){
    $('#hpfill').style.width = Math.max(0, (bossHP/maxHP)*100)+'%';
    $('#gdfill').style.width = Math.max(0, (guard/maxGuard)*100)+'%';
}
function ensureHeroRow(id){
    if (document.getElementById('row-'+id)) return;
    const wrap = document.createElement('div');
    wrap.id = 'row-'+id;
    wrap.innerHTML = `
    <div style="font-weight:600; margin-bottom:4px;">${id}</div>
    <div class="bar" style="height:14px; width:100%; background:#e5e7eb; border-radius:8px; overflow:hidden;">
      <div id="bar-skill-${id}" class="fill" style="height:100%; width:0%; background:#60a5fa;"></div>
      <div id="bar-react-${id}" class="fill" style="height:100%; width:0%; background:#f59e0b;"></div>
    </div>
    <div style="display:flex; justify-content:space-between; font-size:12px; color:#64748b; margin-top:2px;">
      <span id="txt-${id}-skill">技能 0</span>
      <span id="txt-${id}-react">反应 0</span>
      <span id="txt-${id}-total">总计 0</span>
    </div>`;
    $('#dmgs').appendChild(wrap);
}
function renderMeters(){
    let sum = 0; for (const v of dmgByHero.values()) sum += v.total||0;
    for (const [id, v] of dmgByHero.entries()){
        ensureHeroRow(id);
        const tot=v.total||0, sk=v.skill||0, re=v.react||0;
        const pctSk = sum>0 ? (sk/sum*100) : 0;
        const pctRe = sum>0 ? (re/sum*100) : 0;
        $('#bar-skill-'+id).style.width = pctSk+'%';
        $('#bar-react-'+id).style.width = pctRe+'%';
        $('#txt-'+id+'-skill').textContent = `技能 ${fmt0(sk)}`;
        $('#txt-'+id+'-react').textContent = `反应 ${fmt0(re)}`;
        $('#txt-'+id+'-total').textContent = `总计 ${fmt0(tot)}`;
    }
    // Boss 输出
    const bossBox = $('#bossdmg'); let html = '';
    let bossSum = 0; for (const v of bossOutToHero.values()) bossSum += v||0;
    for (const [hid, val] of bossOutToHero.entries()){
        const pct = bossSum>0 ? (val/bossSum*100).toFixed(1) : '0.0';
        html += `<span class="chip">${hid}: ${fmt0(val)} (${pct}%)</span>`;
    }
    bossBox.innerHTML = html || '<span style="color:var(--muted)">暂无</span>';
}

// ===== FX & Utils =====
function stepFX(dt){
    for (let i=fx.length-1;i>=0;i--){ fx[i].life -= dt; if (fx[i].life<=0) fx.splice(i,1); }
    for (let i=moves.length-1;i>=0;i--){ moves[i].life -= dt; if (moves[i].life<=0) moves.splice(i,1); }
}
function spawnFXText(x,y,txt,life=1){
    fx.push({ life, t:0, draw(c){ this.t+=1/60; const a=Math.max(0,1-this.t/life);
            c.globalAlpha=a; c.fillStyle='#111'; c.font='13px system-ui'; c.textAlign='center';
            c.fillText(txt, x, y - this.t*28); c.globalAlpha=1; }});
}
function spawnHitRing(x,y){
    fx.push({ life:0.35, t:0, draw(c){ this.t+=1/60; const a=Math.max(0,1-this.t/0.35); const r=(1+this.t*2)*14;
            c.globalAlpha=a; c.strokeStyle='#f43f5e'; c.lineWidth=2; c.beginPath(); c.arc(x,y,r,0,Math.PI*2); c.stroke(); c.globalAlpha=1; }});
}
function spawnFXBadge(x,y,txt,life=1){
    fx.push({ life, t:0, draw(c){ this.t+=1/60; const a=Math.max(0,1-this.t/life);
            c.globalAlpha=a; c.fillStyle='#fff'; c.strokeStyle='#444'; c.lineWidth=1;
            c.beginPath(); c.roundRect?.(x-12,y-10,24,20,8); if(!c.roundRect){c.rect(x-12,y-10,24,20)}
            c.fill(); c.stroke(); c.fillStyle='#111'; c.font='12px system-ui'; c.textAlign='center'; c.fillText(txt,x,y+5); c.globalAlpha=1; }});
}
function pickColor(id){
    const table = ['#0ea5e9','#22c55e','#f59e0b','#a78bfa','#ef4444'];
    let h=0; for (let i=0;i<id.length;i++) h=(h*131+id.charCodeAt(i))>>>0;
    return table[h%table.length];
}
function badgeFor(s){
    const badges = {
        wet: '💧',
        burning: '🔥',
        frostbite: '❄️',
        slow: '🐢',
        shock: '⚡',
        weaken: '⚠️'
    };
    return badges[s] || '⬢';
}
function fmt(x){ return (x ?? 0).toFixed(2); }
function fmt0(n){ return (n||0).toLocaleString(); }
function toPxX(mx){ return 60 + mx*60 }
function toPxY(my){ return 60 + (10-my)*40 }
function findBoss(){ for (const a of actors.values()) if (a.isBoss) return a; return null; }
function append(s){ const el=document.createElement('div'); el.textContent=s; logBox.appendChild(el); logBox.scrollTop=logBox.scrollHeight; }
function logDetail(ev, text){ append(`[${fmtTime(ev?.t)}] ${text}`); }
function fmtTime(t){
    const totalMs = Math.max(0, Math.round((t ?? 0) * 1000));
    const totalSeconds = Math.floor(totalMs / 1000);
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    const millis = totalMs % 1000;
    const pad2 = n => n.toString().padStart(2, '0');
    const pad3 = n => n.toString().padStart(3, '0');
    return `${pad2(minutes)}:${pad2(seconds)}.${pad3(millis)}`;
}
function loadMeta(meta){
    metaInfo.heroNames.clear();
    metaInfo.heroMaxHp.clear();
    metaInfo.intentNames.clear();
    metaInfo.skillNames.clear();
    metaInfo.bossId = null;
    metaInfo.bossName = null;
    if (!meta) return;
    const heroes = meta.heroes || meta.Heroes || [];
    for (const hero of heroes){
        if (!hero || !hero.id) continue;
        if (hero.name) metaInfo.heroNames.set(hero.id, hero.name);
        const mhp = hero.max_hp ?? hero.maxHp ?? hero.hp;
        if (mhp) metaInfo.heroMaxHp.set(hero.id, mhp);
        if (hero.skills && typeof hero.skills === 'object'){
            for (const [sid, sname] of Object.entries(hero.skills)){
                if (sid && sname) metaInfo.skillNames.set(sid, sname);
            }
        }
    }
    const skillMap = meta.skill_names || meta.skills || meta.SkillNames;
    if (skillMap && typeof skillMap === 'object'){
        for (const [sid, sname] of Object.entries(skillMap)){
            if (sid && sname) metaInfo.skillNames.set(sid, sname);
        }
    }
    const boss = meta.boss || meta.Boss;
    if (boss){
        metaInfo.bossId = boss.id ?? boss.Id ?? null;
        metaInfo.bossName = boss.name ?? boss.Name ?? metaInfo.bossId;
        const bossMaxHp = boss.max_hp ?? boss.maxHp;
        const bossGuard = boss.guard_max ?? boss.guardMax;
        if (bossMaxHp) maxHP = bossMaxHp;
        if (bossGuard) maxGuard = bossGuard;
    }
    const intents = meta.intents || meta.Intents || (boss && boss.intents);
    if (Array.isArray(intents)){
        for (const intent of intents){
            if (!intent || !intent.id) continue;
            metaInfo.intentNames.set(intent.id, intent.name || intent.id);
        }
    }
}
function displayName(id){
    if (!id) return '未知';
    if (metaInfo.heroNames.has(id)) return metaInfo.heroNames.get(id);
    if (id === metaInfo.bossId && metaInfo.bossName) return metaInfo.bossName;
    return id;
}
function actorLabel(id){
    if (!id) return '未知目标';
    const name = displayName(id);
    return name && name !== id ? `${name} (${id})` : name;
}
function skillLabel(id){
    if (!id) return '未知技能';
    return metaInfo.skillNames.get(id) || metaInfo.intentNames.get(id) || id;
}
function statusLabel(id){
    if (!id) return '未知状态';
    return STATUS_LABELS[id] || id;
}
function elemLabel(id){
    if (!id) return '';
    return ELEMENT_LABELS[id] || id;
}
