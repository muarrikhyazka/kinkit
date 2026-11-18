#!/usr/bin/env python3
"""Kiki mascot generator: writes brand/mascot*.svg and the POSES block in react/src/Mascot.jsx."""
import json, re, sys, pathlib

ROOT = pathlib.Path('/home/muarrikh/apps/kinkit')

DEFS = '''<defs>
<radialGradient id="kk-body" cx="0.36" cy="0.30" r="0.80">
<stop offset="0" stop-color="#8B93FA"/><stop offset="0.45" stop-color="#5B54EC"/><stop offset="0.85" stop-color="#4338CA"/><stop offset="1" stop-color="#3730A3"/>
</radialGradient>
<linearGradient id="kk-ao" x1="0" y1="0" x2="0" y2="1">
<stop offset="0.55" stop-color="#312E81" stop-opacity="0"/><stop offset="1" stop-color="#1E1B4B" stop-opacity="0.55"/>
</linearGradient>
<linearGradient id="kk-rim" x1="0" y1="0" x2="1" y2="0">
<stop offset="0.70" stop-color="#C7D2FE" stop-opacity="0"/><stop offset="0.875" stop-color="#C7D2FE" stop-opacity="0.45"/><stop offset="0.93" stop-color="#C7D2FE" stop-opacity="0"/>
</linearGradient>
<radialGradient id="kk-face" cx="0.42" cy="0.32" r="0.75">
<stop offset="0" stop-color="#FFFFFF"/><stop offset="0.6" stop-color="#F5F3FF"/><stop offset="1" stop-color="#D9DDFB"/>
</radialGradient>
<linearGradient id="kk-roof" x1="0" y1="0" x2="0" y2="1">
<stop offset="0" stop-color="#4338CA"/><stop offset="1" stop-color="#312E81"/>
</linearGradient>
<linearGradient id="kk-limb" x1="0" y1="0" x2="1" y2="1">
<stop offset="0" stop-color="#7C83F7"/><stop offset="0.55" stop-color="#4F46E5"/><stop offset="1" stop-color="#3730A3"/>
</linearGradient>
<linearGradient id="kk-foot" x1="0" y1="0" x2="0" y2="1">
<stop offset="0" stop-color="#4338CA"/><stop offset="1" stop-color="#2E2A7A"/>
</linearGradient>
<radialGradient id="kk-eye" cx="0.45" cy="0.35" r="0.7">
<stop offset="0" stop-color="#3B3591"/><stop offset="1" stop-color="#17153B"/>
</radialGradient>
<radialGradient id="kk-cheek" cx="0.5" cy="0.5" r="0.5">
<stop offset="0" stop-color="#F9A8D4" stop-opacity="0.95"/><stop offset="1" stop-color="#F9A8D4" stop-opacity="0"/>
</radialGradient>
<radialGradient id="kk-coin" cx="0.38" cy="0.32" r="0.75">
<stop offset="0" stop-color="#FEF3C7"/><stop offset="0.35" stop-color="#FCD34D"/><stop offset="0.8" stop-color="#F59E0B"/><stop offset="1" stop-color="#B45309"/>
</radialGradient>
<linearGradient id="kk-coinface" x1="0" y1="0" x2="1" y2="1">
<stop offset="0" stop-color="#F59E0B"/><stop offset="1" stop-color="#FDE68A"/>
</linearGradient>
<clipPath id="kk-clip"><path d="BODY"/></clipPath>
<filter id="kk-blur-s" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="5"/></filter>
<filter id="kk-blur-h" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="6"/></filter>
</defs>'''

# Soft house body: gentle peak, rounded shoulders, slightly bulging sides.
BODY = ('M128 50C134 50 139 52.5 144 56.5L188 93C196.5 100 201 107.5 201.5 118'
        'C204 142 204.5 166 202 188C200 205 189 214 171 214L85 214C67 214 56 205 54 188'
        'C51.5 166 52 142 54.5 118C55 107.5 59.5 100 68 93L112 56.5C117 52.5 122 50 128 50Z')

DEFS = DEFS.replace('BODY', BODY)

def limb2(x1, y1, x2, y2, w=11.5):
    import math
    cx, cy = (x1 + x2) / 2, (y1 + y2) / 2
    ry = math.hypot(x2 - x1, y2 - y1) / 2 + w * 0.55
    rot = math.degrees(math.atan2(x2 - x1, -(y2 - y1)))
    return limb(round(cx, 1), round(cy, 1), w, round(ry, 1), round(rot, 1))

def limb(cx, cy, rx, ry, rot):
    return (f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" transform="rotate({rot} {cx} {cy})" fill="url(#kk-limb)"/>'
            f'<ellipse cx="{cx - rx*0.25:.1f}" cy="{cy - ry*0.35:.1f}" rx="{rx*0.35:.1f}" ry="{ry*0.3:.1f}" transform="rotate({rot} {cx} {cy})" fill="#A5B4FC" opacity="0.55"/>')

def coin(cx, cy, r=17):
    s = r / 17
    def p(x, y): return f'{cx + x*s:.1f} {cy + y*s:.1f}'
    return (f'<g class="mk-coin">'
            f'<circle cx="{cx}" cy="{cy + 1.5*s:.1f}" r="{r}" fill="#92400E" opacity="0.35"/>'
            f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="url(#kk-coin)"/>'
            f'<circle cx="{cx}" cy="{cy}" r="{r*0.74:.1f}" fill="url(#kk-coinface)" opacity="0.9"/>'
            f'<path d="M{p(-6.5,0.5)}L{p(0,-5.5)}L{p(6.5,0.5)}M{p(-4.5,-1)}V{cy+6*s:.1f}H{cx+4.5*s:.1f}V{cy-1*s:.1f}" '
            f'fill="none" stroke="#B45309" stroke-width="{2.3*s:.1f}" stroke-linecap="round" stroke-linejoin="round"/>'
            f'<ellipse cx="{cx - r*0.38:.1f}" cy="{cy - r*0.45:.1f}" rx="{r*0.28:.1f}" ry="{r*0.16:.1f}" transform="rotate(-35 {cx - r*0.38:.1f} {cy - r*0.45:.1f})" fill="#FFFFFF" opacity="0.8"/>'
            f'</g>')

def open_eyes(dx=0, dy=0):
    out = '<g class="mk-eyes">'
    for x in (108, 148):
        ex, ey = x + dx, 136 + dy
        out += (f'<ellipse cx="{x}" cy="136" rx="11.5" ry="14" fill="url(#kk-eye)"/>'
                f'<circle cx="{ex + 3.5}" cy="{ey - 5}" r="4.6" fill="#FFFFFF"/>'
                f'<circle cx="{ex - 3.8}" cy="{ey + 5.5}" r="2" fill="#FFFFFF" opacity="0.85"/>')
    return out + '</g>'

def happy_eyes():
    # Upturned arcs (^ ^)
    return ('<g class="mk-eyes" fill="none" stroke="#1E1B4B" stroke-width="5.5" stroke-linecap="round">'
            '<path d="M97 140Q108 125 119 140"/><path d="M137 140Q148 125 159 140"/></g>')

def closed_eyes():
    # Relaxed, downturned lids
    return ('<g class="mk-eyes" fill="none" stroke="#1E1B4B" stroke-width="5" stroke-linecap="round">'
            '<path d="M98 136Q108 145 118 136"/><path d="M138 136Q148 145 158 136"/></g>')

CHEEKS = ('<ellipse cx="89" cy="157" rx="12" ry="7.5" fill="url(#kk-cheek)"/>'
          '<ellipse cx="167" cy="157" rx="12" ry="7.5" fill="url(#kk-cheek)"/>')

SMILE = '<path class="mk-mouth" d="M119 157Q128 166 137 157" fill="none" stroke="#1E1B4B" stroke-width="4.5" stroke-linecap="round"/>'
OPEN_SMILE = ('<g class="mk-mouth"><path d="M116 154Q128 154 140 154Q139 170 128 170Q117 170 116 154Z" fill="#1E1B4B"/>'
              '<path d="M121.5 164.5Q128 160.5 134.5 164.5Q131.5 168 128 168Q124.5 168 121.5 164.5Z" fill="#F472B6"/></g>')
SMALL_MOUTH = '<path class="mk-mouth" d="M120 160Q124 156.5 128 159.5Q132 162.5 136 158.5" fill="none" stroke="#1E1B4B" stroke-width="4.2" stroke-linecap="round"/>'
CONTENT_SMILE = '<path class="mk-mouth" d="M120 156Q128 163 136 156" fill="none" stroke="#1E1B4B" stroke-width="4.2" stroke-linecap="round"/>'

def body(eyes, mouth):
    return ('<g class="mk-body">'
            # feet
            '<ellipse cx="98" cy="213" rx="17" ry="9" fill="url(#kk-foot)"/>'
            '<ellipse cx="158" cy="213" rx="17" ry="9" fill="url(#kk-foot)"/>'
            f'<path d="{BODY}" fill="url(#kk-body)"/>'
            '<g clip-path="url(#kk-clip)">'
            f'<rect x="40" y="40" width="176" height="180" fill="url(#kk-rim)"/>'
            f'<rect x="40" y="40" width="176" height="180" fill="url(#kk-ao)"/>'
            '<ellipse cx="96" cy="96" rx="22" ry="13" transform="rotate(-38 96 96)" fill="#FFFFFF" opacity="0.38" filter="url(#kk-blur-h)"/>'
            '</g>'
            # roof eaves echoing the logo roof line
            '<path d="M60 104L128 47L196 104" fill="none" stroke="url(#kk-roof)" stroke-width="15" stroke-linecap="round" stroke-linejoin="round"/>'
            '<path d="M66 97L126 47.5" fill="none" stroke="#818CF8" stroke-width="3.2" stroke-linecap="round" opacity="0.75"/>'
            '<path d="M130 47.5L170 81" fill="none" stroke="#A5B4FC" stroke-width="2.4" stroke-linecap="round" opacity="0.45"/>'
            # face patch (inner shadow ring + face)
            '<ellipse cx="128" cy="149" rx="55" ry="47" fill="#312E81" opacity="0.18"/>'
            '<ellipse cx="128" cy="151" rx="53" ry="45" fill="url(#kk-face)"/>'
            f'{CHEEKS}{eyes}{mouth}'
            '</g>')

SHADOW = '<ellipse class="mk-shadow" cx="128" cy="226" rx="64" ry="9" fill="#1E1B4B" opacity="0.28" filter="url(#kk-blur-s)"/>'

def arm(cls, inner):
    return f'<g class="mk-arm {cls}">{inner}</g>'

POSES = {}

# idle / wave: left arm resting, right arm raised waving
POSES['idle'] = (SHADOW + '<g class="mk-bob">'
                 + arm('mk-arm-l', limb2(60, 152, 46, 190))
                 + body(open_eyes(), SMILE)
                 + arm('mk-arm-r mk-wave', limb2(194, 158, 222, 116))
                 + '</g>')

# celebrate: both arms up, coin held high on the right
POSES['celebrate'] = (SHADOW + '<g class="mk-bob">'
                      + arm('mk-arm-l', limb2(62, 150, 36, 110))
                      + arm('mk-arm-r', limb2(194, 150, 220, 110))
                      + body(happy_eyes(), OPEN_SMILE)
                      + coin(222, 92, 20)
                      + '<g class="mk-shine" fill="#FBBF24"><path d="M246 64l2.2 5 5 2.2-5 2.2-2.2 5-2.2-5-5-2.2 5-2.2z" opacity="0.9"/></g>'
                      + '</g>')

# relax: eyes closed, arms on belly, small z
POSES['relax'] = (SHADOW + '<g class="mk-bob">'
                  + body(closed_eyes(), CONTENT_SMILE)
                  + arm('mk-arm-l', limb(96, 186, 11, 17, -62))
                  + arm('mk-arm-r', limb(160, 186, 11, 17, 62))
                  + '<g class="mk-zzz" fill="none" stroke="#818CF8" stroke-linecap="round" stroke-linejoin="round">'
                  + '<path d="M196 62h12l-12 13h12" stroke-width="3.6"/>'
                  + '<path d="M218 40h8l-8 9h8" stroke-width="2.8" opacity="0.7"/></g>'
                  + '</g>')

# think: eyes glancing up, hand to chin, thought dots
POSES['think'] = (SHADOW + '<g class="mk-bob">'
                  + arm('mk-arm-l', limb2(60, 152, 46, 190))
                  + body(open_eyes(3, -4), SMALL_MOUTH)
                  + arm('mk-arm-r', limb(156, 176, 11, 17, 58))
                  + '<g class="mk-dots" fill="#A5B4FC">'
                  + '<circle cx="196" cy="70" r="5"/><circle cx="212" cy="52" r="7.5"/><circle cx="234" cy="30" r="10.5"/></g>'
                  + '</g>')

FILES = {'idle': 'mascot.svg', 'celebrate': 'mascot-celebrate.svg', 'relax': 'mascot-relax.svg', 'think': 'mascot-think.svg'}

def minify(s):
    return re.sub(r'>\s+<', '><', s.strip())

defs = minify(DEFS)
for pose, name in FILES.items():
    svg = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="256" height="256" class="mk mk--{pose}">'
           f'<title>Kiki, the Kinkit mascot</title>{defs}{POSES[pose]}</svg>\n')
    (ROOT / 'brand' / name).write_text(svg)

# Inject into Mascot.jsx between markers
jsx = ROOT / 'react/src/Mascot.jsx'
if jsx.exists():
    src = jsx.read_text()
    block = ('// <generated:mascot> (brand/mascot*.svg share this markup; regenerate both together)\n'
             f'const DEFS = {json.dumps(defs)}\n'
             'const POSES = {\n' + ''.join(f'  {k}: {json.dumps(v)},\n' for k, v in POSES.items()) + '}\n'
             '// </generated:mascot>')
    src = re.sub(r'// <generated:mascot>.*?// </generated:mascot>', lambda m: block, src, flags=re.S)
    jsx.write_text(src)
print('ok')
