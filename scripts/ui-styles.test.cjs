const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const css = fs.readFileSync(path.join(__dirname, '..', 'frontend', 'src', 'styles.css'), 'utf8');

function rulesWithReset() {
  return [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].filter(([, , body]) => /border:0;padding:0;box-shadow:none/.test(body));
}

// The SDK Input renders a wrapper div + a raw input. The site's global input rule
// gives that raw input its own border, padding and top margin, so any card that
// composes the SDK input must neutralise it or the fields overlap.
test('the staff login card neutralises the raw input so fields cannot overlap', () => {
  const rules = rulesWithReset();
  assert.ok(rules.length > 0, 'an input reset rule must exist');
  for (const [, selector] of rules) {
    assert.match(selector, /\.staff-login/, `input reset must cover .staff-login, got: ${selector.trim().slice(0, 120)}`);
  }
});

test('the staff login wrapper keeps the shared field box sizing', () => {
  const wrapper = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].filter(([, , body]) => /min-height:46px/.test(body));
  assert.ok(wrapper.length > 0, 'a shared field box rule must exist');
  for (const [, selector] of wrapper) {
    assert.match(selector, /\.staff-login/, `field box sizing must cover .staff-login, got: ${selector.trim().slice(0, 120)}`);
  }
});
