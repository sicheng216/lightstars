import {HyperApi} from "../api/hyper.js";


export class Navigation {
    // {
    //   id: '#xx'.
    //   home: '.'
    // }
    constructor(props) {
        this.id = props.id;
        this.home = props.home;
        this.props = props;

        this.refresh();
    }

    refresh() {
        new HyperApi().get(this, (e) => {
            $(this.id).html(this.render(e.resp));
        });
    }

    render(data) {
        return template.compile(`
        <a class="navbar-brand" href="${this.home}">
            <img src="/static/images/lightstar-6.png" width="30" height="30" alt="">
        </a>
        <button class="navbar-toggler" type="button" data-toggle="collapse"
                data-target="#navbarMore" aria-controls="navbarMore" aria-expanded="false" aria-label="Toggle navigation">
            <span class="navbar-toggler-icon"></span>
        </button>
        <div class="collapse navbar-collapse" id="navbarMore">
            <ul class="navbar-nav mr-auto">
                <li class="nav-item">
                    <a class="nav-link" href="${this.home}#system">Home</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="${this.home}#instances">Guest Instances</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="${this.home}#datastore">DataStore</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="${this.home}#network">Network</a>
                </li>
            </ul>
            <ul class="navbar-nav mr-2">
                <li class="nav-item dropdown">
                    <a class="nav-link dropdown-toggle" href="#" id="navbarMore" data-toggle="dropdown" aria-haspopup="true"
                       aria-expanded="false">
                        {{user.name}}@{{hyper.host}}
                    </a>
                    <div class="dropdown-menu" aria-labelledby="navbarMore">
                        <a class="dropdown-item" href="#">Setting</a>
                        <a class="dropdown-item" href="#">Another action</a>
                        <div class="dropdown-divider"></div>
                        <a class="dropdown-item" href="/ui/login">Logout</a>
                    </div>
                </li>
            </ul>
            <form class="form-inline">
                <input class="form-control form-control-sm" type="search" placeholder="Search" aria-label="Search">
            </form>
        </div>`)(data)
    }
}