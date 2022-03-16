<?php
mb_internal_encoding('UTF-8');

function file_get_contents_utf8($fn) {
     $content = file_get_contents($fn);
      return mb_convert_encoding($content, 'UTF-8',
          mb_detect_encoding($content, 'UTF-8, ISO-8859-1', true));
}

$pagesPath = './ui/src/pages/';

$csv = file_get_contents_utf8('./rezepte.tsv');

$files = glob($pagesPath.'*'); // get all file names
foreach($files as $file){ // iterate files
  if(is_file($file)) {
    unlink($file); // delete file
  }
}

$lines = explode("\n",$csv);
$head = explode("\t", trim($lines[0]));
$jsonObject = array();
$homePage1 = <<<EOD
<template>
  <div class="page" data-name="home">
    <!-- Top Navbar -->
    <div class="navbar">
      <div class="navbar-bg"></div>
      <div class="navbar-inner sliding">
        <div class="title">cocktail tdi
        </div>
      </div>
    </div>
    <!-- Scrollable page content-->
    <div class="page-content">
      <div class="block-title">Zufall</div>
            <div class="list">
              <ul>
                <li>
                  <a href="/Random/" class="item-content item-link">
                    <div class="item-inner">
                      <div class="item-title">Zufall</div>
                    </div>
                  </a>
                </li>
              </ul>
            </div>
      <div class="block-title">Cocktails</div>
      <div class="list">
        <ul>

EOD;

$homePage2 = <<<EOD
        </ul>
      </div>
      <div class="block-title">Pur</div>
      <div class="list">
        <ul>

EOD;

$homePage3 = <<<EOD

        </ul>
      </div>
    </div>
  </div>
</template>
<script>
export default () => {
  return \$render;
}
</script>
EOD;

$routes1 = <<<EOD

import HomePage from '../pages/home.f7';
import Random from '../pages/Random.f7';
//import NotFoundPage from '../pages/404.f7';

EOD;

$routes2 = <<<EOD
var routes = [{path: '/',component: HomePage,},{path: '/Random/',component: Random,},
EOD;

$routes3 = <<<EOD
//{path: '(.*)',component: NotFoundPage,},
];

export default routes;
EOD;

$allIngredients = array();

foreach ($lines as $line) {
    $values = explode("\t", trim($line));
    //print_r($values);
    if($values == $head) {
        continue;
    }
    $ingredients = array();
    $Zutaten = array();
    for ($i = 0;$i < count($values);$i++) {
        //echo $i.'('.$head[$i].'): '.$values[$i]."\n";
        if($head[$i] == 'Cocktail') {
            $name = $values[$i];
        } else if($head[$i] == 'Pre') {
            $pre = $values[$i];
        } else if($head[$i] == 'Post') {
            $post = $values[$i];
        } else if($head[$i] == 'Comment') {
            $comment = $values[$i];
        } else if($head[$i] == 'Wiki') {
            $wiki = $values[$i];
        } else {
            $amount = intval(trim($values[$i],"\n%"));
            if($amount > 0) {
                $ingredients[$head[$i]] = $amount;
                $allIngredients[$head[$i]] = 1;
                $Zutaten[] = array('Name' => $head[$i], 'Menge' => $amount);
            }
        }
    }
    $jsonObject[] = array('Name' => $name, 'Zutaten' => $Zutaten, 'Kommentar' => $comment, 'Vorher' => $pre, 'Nachher' => $post, 'Wiki' => $wiki);
    $cname = str_replace(' ', '', $name);
    
    $routes1 .= 'import '.$cname.' from \'../pages/'.$cname.'.f7\';'."\n";
    $routes2 .= '{path: \'/'.$cname.'/\',component: '.$cname.',},';
    
    $homePage1 .= <<<EOD
            <li>
              <a href="/$cname/" class="item-content item-link">
                <div class="item-inner">
                  <div class="item-title">$name</div>
                </div>
              </a>
            </li>

EOD;
    
    $html = <<<EOD
<template>
  <div class="page">
  <div class="navbar">
    <div class="navbar-bg"></div>
    <div class="navbar-inner sliding">
      <div class="left">
        <a href="#" class="link back">
          <i class="icon icon-back"></i>
          <span class="if-not-md">Back</span>
        </a>
      </div>
      <div class="title">$name</div>
    </div>
  </div>
    <div class="page-content">
      <div class="block-title">Beschreibung</div>
      <div class="block block-strong">
        <p>$comment</p>
      </div>
      <div class="block block-strong"><button class="button button-fill" id="mixit">mix it</button></div>
    </div>
  </div>
</template>
<script>
  export default (props, { $, \$f7, \$on }) => {
    \$on('pageInit', () => {
      $('#mixit').on('click', function() {
        if('$pre' !== ''){\$f7.dialog.alert('$pre');}
        \$f7.dialog.confirm('Bereit? Glas am richtigen Platz?', function () {
          //\$f7.dialog.alert('Great!');
          \$f7.dialog.preloader('$name wird zubereitet ...');
          \$f7.request.get('/ozapftis/$cname').then((res) => {
            setTimeout(function () {
              \$f7.dialog.close();
              if('$post' !== ''){\$f7.dialog.alert('$post');}
            }, 1000);
          });
          /*
          setTimeout(function () {
            \$f7.dialog.close();
            if('$post' !== ''){\$f7.dialog.alert('$post');}
          }, 3000);
          */
        });
      })
    })

    return \$render;
  }
</script>
EOD;
    
    file_put_contents($pagesPath.$cname.'.f7', $html);
}

foreach($allIngredients as $ingredient => $x) {
    $jsonObject[] = array('Name' => $ingredient, 'Zutaten' => array('Name' => $ingredient, 'Menge' => 20), 'Kommentar' => '', 'Pre' => '', 'Post' => '', 'Wiki' => '');
    $routes1 .= 'import '.$ingredient.' from \'../pages/'.$ingredient.'.f7\';'."\n";
    $routes2 .= '{path: \'/'.$ingredient.'/\',component: '.$ingredient.',},';
    
    $homePage2 .= <<<EOD
                <li>
                  <a href="/$ingredient/" class="item-content item-link">
                    <div class="item-inner">
                      <div class="item-title">$ingredient</div>
                    </div>
                  </a>
                </li>
    
EOD;

    $html = <<<EOD
    <template>
      <div class="page">
      <div class="navbar">
        <div class="navbar-bg"></div>
        <div class="navbar-inner sliding">
          <div class="left">
            <a href="#" class="link back">
              <i class="icon icon-back"></i>
              <span class="if-not-md">Back</span>
            </a>
          </div>
          <div class="title">$ingredient</div>
        </div>
      </div>
        <div class="page-content">
          <div class="block-title">Beschreibung</div>
          <div class="block block-strong">
            <p>Einfach nur $ingredient - pur!</p>
          </div>
          <div class="block block-strong"><button class="button button-fill" id="mixit">o'zapft is</button></div>
        </div>
      </div>
    </template>
    <script>
      export default (props, { $, \$f7, \$on }) => {
        /*
        \$on('pageInit', () => {
          $('#mixit').on('click', function() {
            \$f7.dialog.confirm('Bereit? Glas am richtigen Platz?', function () {
              //\$f7.dialog.alert('Great!');
              \$f7.dialog.preloader('$ingredient wird eingefüllt ...');
              setTimeout(function () {
                \$f7.dialog.close();
              }, 3000);
            });
          })
        })
        */
        \$on('pageInit', () => {
          $('#mixit').on('click', function() {
            \$f7.dialog.confirm('Bereit? Glas am richtigen Platz?', function () {
              \$f7.dialog.preloader('$ingredient wird eingefüllt ...');
              \$f7.request.get('/ozapftis/$ingredient').then((res) => {
                setTimeout(function () {
                  \$f7.dialog.close();
                }, 1000);
              });
            });
          })
        })
    
        return \$render;
      }
    </script>
EOD;
    file_put_contents($pagesPath.$ingredient.'.f7', $html);
}

$html = <<<EOD
    <template>
      <div class="page">
      <div class="navbar">
        <div class="navbar-bg"></div>
        <div class="navbar-inner sliding">
          <div class="left">
            <a href="#" class="link back">
              <i class="icon icon-back"></i>
              <span class="if-not-md">Back</span>
            </a>
          </div>
          <div class="title">Zufall</div>
        </div>
      </div>
        <div class="page-content">
          <div class="block-title">Beschreibung</div>
          <div class="block block-strong">
            <p>mal schauen was da kommt</p>
          </div>
          <div class="block block-strong"><button class="button button-fill" id="mixit">Zufall</button></div>
        </div>
      </div>
    </template>
    <script>
      export default (props, { $, \$f7, \$on }) => {
        \$on('pageInit', () => {
          $('#mixit').on('click', function() {
            \$f7.dialog.confirm('Bereit? Glas am richtigen Platz?', function () {
              \$f7.dialog.preloader('Irgendwas wird eingefüllt ...');
              \$f7.request.get('/ozapftis/random').then((res) => {
                setTimeout(function () {
                  \$f7.dialog.close();
                }, 1000);
              });
            });
          })
        })

        return \$render;
      }
    </script>
EOD;

file_put_contents($pagesPath.'Random.f7', $html);

file_put_contents($pagesPath.'../js/routes.js', $routes1.$routes2.$routes3);
file_put_contents($pagesPath.'home.f7', $homePage1.$homePage2.$homePage3);

//print_r($jsonObject);

$json = json_encode($jsonObject, JSON_PRETTY_PRINT);
switch(json_last_error()) {
    case JSON_ERROR_NONE:
        echo ' - Keine Fehler';
    break;
    case JSON_ERROR_DEPTH:
        echo ' - Maximale Stacktiefe überschritten';
    break;
    case JSON_ERROR_STATE_MISMATCH:
        echo ' - Unterlauf oder Nichtübereinstimmung der Modi';
    break;
    case JSON_ERROR_CTRL_CHAR:
        echo ' - Unerwartetes Steuerzeichen gefunden';
    break;
    case JSON_ERROR_SYNTAX:
        echo ' - Syntaxfehler, ungültiges JSON';
    break;
    case JSON_ERROR_UTF8:
        echo ' - Missgestaltete UTF-8 Zeichen, möglicherweise fehlerhaft kodiert';
    break;
    default:
        echo ' - Unbekannter Fehler';
    break;
}
//print_r($json);

file_put_contents('./rezepte.json', $json);

?>