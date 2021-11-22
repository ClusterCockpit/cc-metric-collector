Name:           cc-metric-collector
Version:        0.1
Release:        1%{?dist}
Summary:        Metric collection daemon from the ClusterCockpit suite

License:        MIT
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang
BuildRequires:  systemd-rpm-macros

Provides:       %{name} = %{version}

%description
A simple web app

%global debug_package %{nil}

%prep
%autosetup


%build
cd collectors
make
go build -v -o %{name}


%install
install -Dpm 0755 %{name} %{buildroot}%{_bindir}/%{name}
install -Dpm 0755 config.json %{buildroot}%{_sysconfdir}/%{name}/config.json
install -Dpm 644 scripts/%{name}.service %{buildroot}%{_unitdir}/%{name}.service

%check
# go test should be here... :)

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%files
%dir %{_sysconfdir}/%{name}
%{_bindir}/%{name}
%{_unitdir}/%{name}.service
%config(noreplace) %{_sysconfdir}/%{name}/config.json


%changelog
* Mon Nov 22 2021 Thomas Gruber - 0.1
- Initial spec file
